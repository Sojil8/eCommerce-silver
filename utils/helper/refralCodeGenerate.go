package helper

import "math/rand"



func GenerateReferralCode() (string, error) {
	length:=6
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	code := make([]byte, length)
	_, err := rand.Read(code)
	if err != nil {
		return "", err
	}

	for i := 0; i < length; i++ {
		code[i] = charset[int(code[i])%len(charset)]
	}
	return string(code), nil
}
