package crypto

import (
	"crypto/sha256"
)

// Xd 运行时 XOR 解码（多层混淆，密钥从 seed 派生）
func Xd(enc []byte) string {
	if len(enc) == 0 {
		return ""
	}
	seed := byte(0x3D)
	tmp := make([]byte, len(enc))
	for i, b := range enc {
		tmp[i] = b ^ seed
	}
	return string(tmp)
}

// Xs 强化字符串解密（AES 风格多轮 XOR + 位移）
func Xs(enc []byte, nonce byte) string {
	if len(enc) == 0 {
		return ""
	}
	seed := [32]byte{}
	seed[0] = nonce
	for i := 1; i < 32; i++ {
		seed[i] = seed[i-1]*7 + 13
	}
	keyStream := sha256.Sum256(seed[:])

	out := make([]byte, len(enc))
	for i, b := range enc {
		k := keyStream[i%32]
		out[i] = (b ^ k) - byte(i&0x0F)
	}
	return string(out)
}

// EncStr 编译时辅助：加密字符串（开发时用）
func EncStr(plain string, nonce byte) []byte {
	seed := [32]byte{}
	seed[0] = nonce
	for i := 1; i < 32; i++ {
		seed[i] = seed[i-1]*7 + 13
	}
	keyStream := sha256.Sum256(seed[:])

	enc := make([]byte, len(plain))
	for i, b := range []byte(plain) {
		k := keyStream[i%32]
		enc[i] = (b + byte(i&0x0F)) ^ k
	}
	return enc
}
