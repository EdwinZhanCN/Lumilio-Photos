// Package secretbox seals small application secrets (cloud credentials and
// encrypted settings values) with AES keys derived per scope from one root secret.
// [LoadOrCreateLumilioSecretKey] reads or creates the owner-only root key file
// named by the manifest; [New] and [NewFromRoot] build a scoped [Box].
//
//atlas:group foundation
package secretbox
