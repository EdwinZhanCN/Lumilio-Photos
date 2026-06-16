package icloud

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"

	"golang.org/x/crypto/pbkdf2"
)

var (
	// RFC 5054 2048-bit group
	srpN, _ = new(big.Int).SetString(
		"AC6BDB41324A9A9BF166DE5E1389582FAF72B6651987EE07FC3192943DB56050"+
			"A37329CBB4A099ED8193E0757767A13DD52312AB4B03310DCD7F48A9DA04FD50"+
			"E8083969EDB767B0CF6095179A163AB3661A05FBD5FAAAE82918A9962F0B93B8"+
			"55F97993EC975EEAA80D740ADBF4FF747359D041D5C33EA71D281E446B14773B"+
			"CA97B43A23FB801676BD207A436C6481F1D2B9078717461A5B9D32E688F87748"+
			"544523B524B0D57D5EA77A2775D2ECFA032CFBDBF52FB3786160279004E57AE6"+
			"AF874E7303CE53299CCC041C7BC308D82A5698F3A8D0C38271AE35F8E9DBFBB6"+
			"94B5C803D89F7AE435DE236D525F54759B65E372FCD68EF20FA7111F9E4AFF73", 16)
	srpG = big.NewInt(2)
	srpK *big.Int
)

func init() {
	// k = H(N, pad(g))
	srpK = srpComputeK()
}

func srpComputeK() *big.Int {
	nBytes := srpN.Bytes()
	gBytes := padTo(srpG.Bytes(), len(nBytes))
	h := sha256.New()
	h.Write(nBytes)
	h.Write(gBytes)
	return new(big.Int).SetBytes(h.Sum(nil))
}

func padTo(b []byte, length int) []byte {
	if len(b) >= length {
		return b
	}
	padded := make([]byte, length)
	copy(padded[length-len(b):], b)
	return padded
}

func srpComputeU(A, B *big.Int) *big.Int {
	nLen := len(srpN.Bytes())
	aBytes := padTo(A.Bytes(), nLen)
	bBytes := padTo(B.Bytes(), nLen)
	h := sha256.New()
	h.Write(aBytes)
	h.Write(bBytes)
	return new(big.Int).SetBytes(h.Sum(nil))
}

func srpDerivePassword(password string, salt []byte, iterations int, protocol string) []byte {
	passHash := sha256.Sum256([]byte(password))
	var passDigest []byte
	if protocol == "s2k_fo" {
		passDigest = []byte(hex.EncodeToString(passHash[:]))
	} else {
		passDigest = passHash[:]
	}
	return pbkdf2.Key(passDigest, salt, iterations, 32, sha256.New)
}

func srpComputeX(salt []byte, derivedPassword []byte) *big.Int {
	// no_username_in_x: x = H(salt, H(":" + derivedPassword))
	inner := sha256.New()
	inner.Write([]byte(":"))
	inner.Write(derivedPassword)
	innerHash := inner.Sum(nil)

	h := sha256.New()
	h.Write(salt)
	h.Write(innerHash)
	return new(big.Int).SetBytes(h.Sum(nil))
}

func srpComputeClientSession(a, B, x, u *big.Int) *big.Int {
	// S = (B - k * g^x) ^ (a + u * x) % N
	gx := new(big.Int).Exp(srpG, x, srpN)
	kgx := new(big.Int).Mul(srpK, gx)
	kgx.Mod(kgx, srpN)

	diff := new(big.Int).Sub(B, kgx)
	if diff.Sign() < 0 {
		diff.Add(diff, srpN)
	}

	exp := new(big.Int).Mul(u, x)
	exp.Add(exp, a)

	S := new(big.Int).Exp(diff, exp, srpN)
	return S
}

func srpComputeHAMK(A *big.Int, M []byte, K []byte) []byte {
	h := sha256.New()
	h.Write(A.Bytes())
	h.Write(M)
	h.Write(K)
	return h.Sum(nil)
}

func srpSessionKey(S *big.Int) []byte {
	h := sha256.New()
	h.Write(S.Bytes())
	return h.Sum(nil)
}

type srpInitResponse struct {
	Salt      string `json:"salt"`
	B         string `json:"b"`
	C         string `json:"c"`
	Iteration int    `json:"iteration"`
	Protocol  string `json:"protocol"`
}

func (r *Client) signInSRP(password string) error {
	// Step 1: Generate ephemeral key pair (256 bytes = 2048 bits, matching pysrp)
	aBytes := make([]byte, 256)
	if _, err := rand.Read(aBytes); err != nil {
		return fmt.Errorf("generate random failed: %w", err)
	}
	aBytes[0] |= 0x80 // ensure MSB is set (get_random_of_length behavior)
	a := new(big.Int).SetBytes(aBytes)
	A := new(big.Int).Exp(srpG, a, srpN)

	aBase64 := base64.StdEncoding.EncodeToString(A.Bytes())

	// Step 2: Send init request
	initBody := map[string]any{
		"a":           aBase64,
		"accountName": r.appleID,
		"protocols":   []string{"s2k", "s2k_fo"},
	}

	headers := r.getAuthHeaders(map[string]string{
		"Origin":  r.authRootEndpoint,
		"Referer": r.authRootEndpoint + "/",
	})

	initText, err := r.request(&rawReq{
		Method:       http.MethodPost,
		URL:          r.authEndpoint + "/signin/init",
		Headers:      headers,
		Body:         initBody,
		ExpectStatus: newSet[int](http.StatusOK),
	})
	if err != nil {
		return fmt.Errorf("SRP signin init failed: %w", err)
	}

	// Step 3: Parse server response
	var initResp srpInitResponse
	if err := json.Unmarshal([]byte(initText), &initResp); err != nil {
		return fmt.Errorf("SRP signin init unmarshal failed: %w, text: %s", err, initText)
	}

	salt, err := base64.StdEncoding.DecodeString(initResp.Salt)
	if err != nil {
		return fmt.Errorf("decode salt failed: %w", err)
	}
	bBytes2, err := base64.StdEncoding.DecodeString(initResp.B)
	if err != nil {
		return fmt.Errorf("decode B failed: %w", err)
	}
	B := new(big.Int).SetBytes(bBytes2)

	// Verify B % N != 0
	if new(big.Int).Mod(B, srpN).Sign() == 0 {
		return fmt.Errorf("SRP: server sent invalid B value")
	}

	// Step 4: Compute session key
	u := srpComputeU(A, B)
	if u.Sign() == 0 {
		return fmt.Errorf("SRP: computed u is zero")
	}

	derivedPassword := srpDerivePassword(password, salt, initResp.Iteration, initResp.Protocol)
	x := srpComputeX(salt, derivedPassword)
	S := srpComputeClientSession(a, B, x, u)
	K := srpSessionKey(S)

	// Step 5: Compute M1 and M2 (HAMK)
	m1 := computeM1Full(A, B, K, r.appleID, salt)
	m2 := srpComputeHAMK(A, m1, K)

	// Step 6: Send complete request
	completeBody := map[string]any{
		"accountName": r.appleID,
		"c":           initResp.C,
		"m1":          base64.StdEncoding.EncodeToString(m1),
		"m2":          base64.StdEncoding.EncodeToString(m2),
		"rememberMe":  true,
		"trustTokens": []string{},
	}
	if r.sessionData.TrustToken != "" {
		completeBody["trustTokens"] = []string{r.sessionData.TrustToken}
	}

	_, status, err := r.requestWithStatus(&rawReq{
		Method:       http.MethodPost,
		URL:          r.authEndpoint + "/signin/complete",
		Headers:      headers,
		Querys:       map[string]string{"isRememberMeEnabled": "true"},
		Body:         completeBody,
		ExpectStatus: newSet[int](http.StatusOK, http.StatusConflict, http.StatusPreconditionFailed),
	})
	if err != nil {
		r.debugf("srp signin complete failed apple_id=%s err=%v", maskAppleID(r.appleID), err)
		return fmt.Errorf("SRP signin complete failed: %w", err)
	}

	r.debugf(
		"srp signin complete apple_id=%s status=%d session_token=%t trust_token=%t scnt=%t session_id=%t",
		maskAppleID(r.appleID),
		status,
		r.sessionData.SessionToken != "",
		r.sessionData.TrustToken != "",
		r.sessionData.Scnt != "",
		r.sessionData.SessionID != "",
	)

	// 412: non-2FA account, needs repair
	if status == http.StatusPreconditionFailed {
		repairHeaders := r.getAuthHeaders(map[string]string{})
		_, repairErr := r.request(&rawReq{
			Method:       http.MethodPost,
			URL:          r.authEndpoint + "/repair/complete",
			Headers:      repairHeaders,
			Body:         map[string]any{},
			ExpectStatus: newSet[int](http.StatusOK),
		})
		if repairErr != nil {
			r.debugf("srp repair complete failed apple_id=%s err=%v", maskAppleID(r.appleID), repairErr)
			return fmt.Errorf("SRP repair complete failed: %w", repairErr)
		}
		r.debugf("srp repair complete succeeded apple_id=%s", maskAppleID(r.appleID))
	}

	return nil
}

// computeM1Full computes M1 using the full SRP-6a formula:
// M1 = H( H(N) XOR H(g) | H(I) | salt | A | B | K )
func computeM1Full(A, B *big.Int, K []byte, username string, salt []byte) []byte {
	nBytes := srpN.Bytes()
	// H(N)
	hN := sha256.Sum256(nBytes)
	// H(g) - with rfc5054_compat, g is padded to N's width before hashing
	hG := sha256.Sum256(padTo(srpG.Bytes(), len(nBytes)))
	// H(N) XOR H(g)
	hNxorHg := make([]byte, 32)
	for i := range hNxorHg {
		hNxorHg[i] = hN[i] ^ hG[i]
	}

	// H(I) - username
	hI := sha256.Sum256([]byte(username))

	h := sha256.New()
	h.Write(hNxorHg)
	h.Write(hI[:])
	h.Write(salt)
	h.Write(A.Bytes())
	h.Write(B.Bytes())
	h.Write(K)
	return h.Sum(nil)
}
