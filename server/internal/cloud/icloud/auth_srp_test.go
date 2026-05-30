package icloud

import (
	"crypto/sha256"
	"encoding/hex"
	"math/big"
	"testing"
)

func TestSRPComputeK(t *testing.T) {
	k := srpComputeK()
	expected := "05b9e8ef059c6b32ea59fc1d322d37f04aa30bae5aa9003b8321e21ddb04e300"
	got := hex.EncodeToString(k.Bytes())
	if got != expected {
		t.Errorf("k mismatch\n  got:  %s\n  want: %s", got, expected)
	}
}

func TestSRPComputeX(t *testing.T) {
	salt := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10}
	password := []byte("test_derived_password_32_bytes!!")

	x := srpComputeX(salt, password)
	expected := "038334a732f9f02976134d9ff8c2f68c7d203785eba3d6fb91749352d7141ffd"
	got := hex.EncodeToString(padTo(x.Bytes(), 32))
	if got != expected {
		t.Errorf("x mismatch\n  got:  %s\n  want: %s", got, expected)
	}
}

func TestHNxorg(t *testing.T) {
	nBytes := srpN.Bytes()
	hN := sha256.Sum256(nBytes)
	hG := sha256.Sum256(padTo(srpG.Bytes(), len(nBytes)))
	hNxorHg := make([]byte, 32)
	for i := range hNxorHg {
		hNxorHg[i] = hN[i] ^ hG[i]
	}
	expected := "a2b80ee3d957ede6b072f76f67415f3d1a7158bb8f0e765e34d87844031acc9c"
	got := hex.EncodeToString(hNxorHg)
	if got != expected {
		t.Errorf("HNxorg mismatch\n  got:  %s\n  want: %s", got, expected)
	}
}

func TestSRPComputeU(t *testing.T) {
	aHex := ""
	for i := 0; i < 32; i++ {
		aHex += "1234567890abcdef"
	}
	aPrivate, _ := new(big.Int).SetString(aHex, 16)
	A := new(big.Int).Exp(srpG, aPrivate, srpN)
	B := new(big.Int).Exp(srpG, big.NewInt(42), srpN)

	u := srpComputeU(A, B)
	expected := "f6a777afb8963b4af2001cc254190286db6db510e98a9bcbba161a6ffb8f530d"
	got := hex.EncodeToString(u.Bytes())
	if got != expected {
		t.Errorf("u mismatch\n  got:  %s\n  want: %s", got, expected)
	}
}

func TestSRPDerivePassword(t *testing.T) {
	salt, _ := hex.DecodeString("aabbccdd")
	iterations := 20000

	derivedFo := srpDerivePassword("mypassword123", salt, iterations, "s2k_fo")
	expectedFo := "df8070ac89e2f3d00113b10571052e55c25f93f90bb963aba5ecf33ee908f2ba"
	if hex.EncodeToString(derivedFo) != expectedFo {
		t.Errorf("s2k_fo derived mismatch\n  got:  %s\n  want: %s", hex.EncodeToString(derivedFo), expectedFo)
	}

	derivedS2k := srpDerivePassword("mypassword123", salt, iterations, "s2k")
	expectedS2k := "841605beabadd02590085d9679354380275d3cc7120cbd2607d03d3df37e8950"
	if hex.EncodeToString(derivedS2k) != expectedS2k {
		t.Errorf("s2k derived mismatch\n  got:  %s\n  want: %s", hex.EncodeToString(derivedS2k), expectedS2k)
	}

	xFo := srpComputeX(salt, derivedFo)
	expectedXFo := "19fa046ba0410b4eaad5e3c162b418a790589091c3b6787d3951b3b0faecda77"
	if hex.EncodeToString(padTo(xFo.Bytes(), 32)) != expectedXFo {
		t.Errorf("x (s2k_fo) mismatch\n  got:  %s\n  want: %s", hex.EncodeToString(padTo(xFo.Bytes(), 32)), expectedXFo)
	}

	xS2k := srpComputeX(salt, derivedS2k)
	expectedXS2k := "a5feba245cfdf5c17a0b8ce00a4fc7fb0b5ae1dff38122abbd88fcd64fd3311a"
	if hex.EncodeToString(padTo(xS2k.Bytes(), 32)) != expectedXS2k {
		t.Errorf("x (s2k) mismatch\n  got:  %s\n  want: %s", hex.EncodeToString(padTo(xS2k.Bytes(), 32)), expectedXS2k)
	}
}

func TestSRPFullFlow(t *testing.T) {
	aHex := ""
	for i := 0; i < 32; i++ {
		aHex += "1234567890abcdef"
	}
	aPrivate, _ := new(big.Int).SetString(aHex, 16)
	A := new(big.Int).Exp(srpG, aPrivate, srpN)
	B := new(big.Int).Exp(srpG, big.NewInt(42), srpN)

	salt := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10}
	password := []byte("test_derived_password_32_bytes!!")

	u := srpComputeU(A, B)
	x := srpComputeX(salt, password)
	S := srpComputeClientSession(aPrivate, B, x, u)
	K := srpSessionKey(S)

	expectedK := "e3ff93b7441e52dfe2cf5a688635e3840392384a9c195139e410bb0b156e190a"
	gotK := hex.EncodeToString(K)
	if gotK != expectedK {
		t.Errorf("K mismatch\n  got:  %s\n  want: %s", gotK, expectedK)
	}

	m1 := computeM1Full(A, B, K, "testuser", salt)
	expectedM1 := "2f7688aa2575033ff4b814c653664530e7f27ee86f727b8e29ffeedbbc583b3b"
	gotM1 := hex.EncodeToString(m1)
	if gotM1 != expectedM1 {
		t.Errorf("M1 mismatch\n  got:  %s\n  want: %s", gotM1, expectedM1)
	}

	hamk := srpComputeHAMK(A, m1, K)
	expectedHAMK := "c26647c64b59ffd995ac76dfa497868cb14f0cd9b6af95719c9457c4c498b716"
	gotHAMK := hex.EncodeToString(hamk)
	if gotHAMK != expectedHAMK {
		t.Errorf("H_AMK mismatch\n  got:  %s\n  want: %s", gotHAMK, expectedHAMK)
	}
}
