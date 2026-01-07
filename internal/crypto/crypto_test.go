package crypto

import (
	"bytes"
	"testing"
)

func TestGenerateKey(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	if len(key) != KeyLength {
		t.Errorf("GenerateKey() length = %d, want %d", len(key), KeyLength)
	}

	// Keys should be unique
	key2, _ := GenerateKey()
	if bytes.Equal(key, key2) {
		t.Error("GenerateKey() generated duplicate keys")
	}
}

func TestGenerateSalt(t *testing.T) {
	salt, err := GenerateSalt()
	if err != nil {
		t.Fatalf("GenerateSalt() error = %v", err)
	}
	if len(salt) != SaltLength {
		t.Errorf("GenerateSalt() length = %d, want %d", len(salt), SaltLength)
	}

	// Salts should be unique
	salt2, _ := GenerateSalt()
	if bytes.Equal(salt, salt2) {
		t.Error("GenerateSalt() generated duplicate salts")
	}
}

func TestGenerateNonce(t *testing.T) {
	nonce, err := GenerateNonce()
	if err != nil {
		t.Fatalf("GenerateNonce() error = %v", err)
	}
	if len(nonce) != NonceLength {
		t.Errorf("GenerateNonce() length = %d, want %d", len(nonce), NonceLength)
	}

	// Nonces should be unique
	nonce2, _ := GenerateNonce()
	if bytes.Equal(nonce, nonce2) {
		t.Error("GenerateNonce() generated duplicate nonces")
	}
}

func TestEncryptDecrypt(t *testing.T) {
	key, _ := GenerateKey()
	plaintext := []byte("super secret password")
	aad := []byte("DATABASE_URL") // Additional authenticated data

	// Encrypt
	ciphertext, nonce, err := Encrypt(key, plaintext, aad)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	// Ciphertext should not be empty
	if len(ciphertext) == 0 {
		t.Error("Encrypt() returned empty ciphertext")
	}

	// Ciphertext should be different from plaintext
	if bytes.Equal(ciphertext, plaintext) {
		t.Error("Encrypt() ciphertext equals plaintext")
	}

	// Decrypt
	decrypted, err := Decrypt(key, ciphertext, nonce, aad)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}

	// Decrypted should match original
	if !bytes.Equal(decrypted, plaintext) {
		t.Errorf("Decrypt() = %v, want %v", decrypted, plaintext)
	}
}

func TestEncryptDecryptString(t *testing.T) {
	key, _ := GenerateKey()
	plaintext := "my secret value"
	aad := []byte("API_KEY")

	// Encrypt
	ciphertext, nonce, err := EncryptString(key, plaintext, aad)
	if err != nil {
		t.Fatalf("EncryptString() error = %v", err)
	}

	// Decrypt
	decrypted, err := DecryptString(key, ciphertext, nonce, aad)
	if err != nil {
		t.Fatalf("DecryptString() error = %v", err)
	}

	if decrypted != plaintext {
		t.Errorf("DecryptString() = %v, want %v", decrypted, plaintext)
	}
}

func TestDecryptWrongKey(t *testing.T) {
	key1, _ := GenerateKey()
	key2, _ := GenerateKey()
	plaintext := []byte("secret")
	aad := []byte("KEY")

	// Encrypt with key1
	ciphertext, nonce, _ := Encrypt(key1, plaintext, aad)

	// Try to decrypt with key2
	_, err := Decrypt(key2, ciphertext, nonce, aad)
	if err != ErrDecryptionFailed {
		t.Errorf("Decrypt() with wrong key error = %v, want ErrDecryptionFailed", err)
	}
}

func TestDecryptWrongAAD(t *testing.T) {
	key, _ := GenerateKey()
	plaintext := []byte("secret")
	aad1 := []byte("KEY1")
	aad2 := []byte("KEY2")

	// Encrypt with aad1
	ciphertext, nonce, _ := Encrypt(key, plaintext, aad1)

	// Try to decrypt with aad2
	_, err := Decrypt(key, ciphertext, nonce, aad2)
	if err != ErrDecryptionFailed {
		t.Errorf("Decrypt() with wrong AAD error = %v, want ErrDecryptionFailed", err)
	}
}

func TestDecryptCorruptedCiphertext(t *testing.T) {
	key, _ := GenerateKey()
	plaintext := []byte("secret")

	ciphertext, nonce, _ := Encrypt(key, plaintext, nil)

	// Corrupt the ciphertext
	ciphertext[0] ^= 0xFF

	_, err := Decrypt(key, ciphertext, nonce, nil)
	if err != ErrDecryptionFailed {
		t.Errorf("Decrypt() with corrupted ciphertext error = %v, want ErrDecryptionFailed", err)
	}
}

func TestEncryptInvalidKeyLength(t *testing.T) {
	shortKey := make([]byte, 16) // 128 bits instead of 256
	plaintext := []byte("secret")

	_, _, err := Encrypt(shortKey, plaintext, nil)
	if err != ErrInvalidKeyLength {
		t.Errorf("Encrypt() with short key error = %v, want ErrInvalidKeyLength", err)
	}
}

func TestDecryptInvalidKeyLength(t *testing.T) {
	shortKey := make([]byte, 16)
	ciphertext := []byte("fake ciphertext")
	nonce := make([]byte, NonceLength)

	_, err := Decrypt(shortKey, ciphertext, nonce, nil)
	if err != ErrInvalidKeyLength {
		t.Errorf("Decrypt() with short key error = %v, want ErrInvalidKeyLength", err)
	}
}

func TestDecryptInvalidNonceLength(t *testing.T) {
	key, _ := GenerateKey()
	ciphertext := []byte("fake ciphertext")
	shortNonce := make([]byte, 8) // Should be 12

	_, err := Decrypt(key, ciphertext, shortNonce, nil)
	if err != ErrInvalidNonceLength {
		t.Errorf("Decrypt() with short nonce error = %v, want ErrInvalidNonceLength", err)
	}
}

func TestEncryptEmptyPlaintext(t *testing.T) {
	key, _ := GenerateKey()
	plaintext := []byte("")

	ciphertext, nonce, err := Encrypt(key, plaintext, nil)
	if err != nil {
		t.Fatalf("Encrypt() with empty plaintext error = %v", err)
	}

	decrypted, err := Decrypt(key, ciphertext, nonce, nil)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Errorf("Decrypt() = %v, want empty", decrypted)
	}
}

func TestEncryptLargePlaintext(t *testing.T) {
	key, _ := GenerateKey()
	// 1 MB of data
	plaintext := make([]byte, 1024*1024)
	for i := range plaintext {
		plaintext[i] = byte(i % 256)
	}

	ciphertext, nonce, err := Encrypt(key, plaintext, nil)
	if err != nil {
		t.Fatalf("Encrypt() with large plaintext error = %v", err)
	}

	decrypted, err := Decrypt(key, ciphertext, nonce, nil)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Error("Decrypt() large plaintext mismatch")
	}
}

func TestDeriveKey(t *testing.T) {
	password := "my-secure-password"
	salt, _ := GenerateSalt()

	key := DeriveKey(password, salt)
	if len(key) != KeyLength {
		t.Errorf("DeriveKey() length = %d, want %d", len(key), KeyLength)
	}

	// Same password and salt should produce same key
	key2 := DeriveKey(password, salt)
	if !bytes.Equal(key, key2) {
		t.Error("DeriveKey() produced different keys for same inputs")
	}

	// Different password should produce different key
	key3 := DeriveKey("different-password", salt)
	if bytes.Equal(key, key3) {
		t.Error("DeriveKey() produced same key for different passwords")
	}

	// Different salt should produce different key
	salt2, _ := GenerateSalt()
	key4 := DeriveKey(password, salt2)
	if bytes.Equal(key, key4) {
		t.Error("DeriveKey() produced same key for different salts")
	}
}

func TestDeriveKeyWithParams(t *testing.T) {
	password := "test-password"
	salt, _ := GenerateSalt()

	// Use minimal parameters for fast testing
	key := DeriveKeyWithParams(password, salt, 1, 64*1024, 1)
	if len(key) != KeyLength {
		t.Errorf("DeriveKeyWithParams() length = %d, want %d", len(key), KeyLength)
	}

	// Same inputs should produce same key
	key2 := DeriveKeyWithParams(password, salt, 1, 64*1024, 1)
	if !bytes.Equal(key, key2) {
		t.Error("DeriveKeyWithParams() produced different keys for same inputs")
	}
}

func TestDeriveKeyUsableForEncryption(t *testing.T) {
	password := "my-password"
	salt, _ := GenerateSalt()

	// Derive key from password
	key := DeriveKey(password, salt)

	// Use derived key for encryption
	plaintext := []byte("secret data")
	ciphertext, nonce, err := Encrypt(key, plaintext, nil)
	if err != nil {
		t.Fatalf("Encrypt() with derived key error = %v", err)
	}

	// Re-derive key and decrypt
	key2 := DeriveKey(password, salt)
	decrypted, err := Decrypt(key2, ciphertext, nonce, nil)
	if err != nil {
		t.Fatalf("Decrypt() with re-derived key error = %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Errorf("Decrypt() = %v, want %v", decrypted, plaintext)
	}
}

// Benchmark tests
func BenchmarkEncrypt(b *testing.B) {
	key, _ := GenerateKey()
	plaintext := []byte("benchmark secret value")
	aad := []byte("KEY")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Encrypt(key, plaintext, aad)
	}
}

func BenchmarkDecrypt(b *testing.B) {
	key, _ := GenerateKey()
	plaintext := []byte("benchmark secret value")
	aad := []byte("KEY")
	ciphertext, nonce, _ := Encrypt(key, plaintext, aad)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Decrypt(key, ciphertext, nonce, aad)
	}
}

func BenchmarkDeriveKey(b *testing.B) {
	password := "benchmark-password"
	salt, _ := GenerateSalt()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		DeriveKey(password, salt)
	}
}
