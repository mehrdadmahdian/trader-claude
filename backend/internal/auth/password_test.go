package auth

import (
	"testing"
)

func TestHashAndCompare_Success(t *testing.T) {
	hash, err := HashPassword("SecurePass1")
	if err != nil {
		t.Fatalf("hash error: %v", err)
	}
	if err := ComparePassword(hash, "SecurePass1"); err != nil {
		t.Fatalf("compare should succeed: %v", err)
	}
}

func TestHashAndCompare_WrongPassword(t *testing.T) {
	hash, err := HashPassword("SecurePass1")
	if err != nil {
		t.Fatalf("hash error: %v", err)
	}
	if err := ComparePassword(hash, "WrongPass1"); err == nil {
		t.Fatal("compare should fail for wrong password")
	}
}

func TestValidatePasswordPolicy_Valid(t *testing.T) {
	valid := []string{"SecurePass1", "Ab1defgh", "MyP@ss1word"}
	for _, p := range valid {
		if err := ValidatePasswordPolicy(p); err != nil {
			t.Errorf("expected %q to be valid, got: %v", p, err)
		}
	}
}

func TestValidatePasswordPolicy_TooShort(t *testing.T) {
	if err := ValidatePasswordPolicy("Ab1"); err == nil {
		t.Fatal("expected error for short password")
	}
}

func TestValidatePasswordPolicy_NoUppercase(t *testing.T) {
	if err := ValidatePasswordPolicy("securepass1"); err == nil {
		t.Fatal("expected error for no uppercase")
	}
}

func TestValidatePasswordPolicy_NoLowercase(t *testing.T) {
	if err := ValidatePasswordPolicy("SECUREPASS1"); err == nil {
		t.Fatal("expected error for no lowercase")
	}
}

func TestValidatePasswordPolicy_NoDigit(t *testing.T) {
	if err := ValidatePasswordPolicy("SecurePass"); err == nil {
		t.Fatal("expected error for no digit")
	}
}
