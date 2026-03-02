package auth

/**
 * @Description:jwt认证
 */

import (
	"GoChat/config"
	"errors"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
)

type CustomClaims struct {
	UserID uint64 `json:"user_id"`
	Email  string `json:"email"`
	Phone  string `json:"phone"`
	State  byte   `json:"state"` // 例如 "admin", "user"
	jwt.RegisteredClaims
}

// 定义常见错误，方便上层判断
var (
	ErrTokenExpired     = errors.New("token is expired")
	ErrTokenNotValidYet = errors.New("token not active yet")
	ErrTokenMalformed   = errors.New("that's not even a token")
	ErrTokenInvalid     = errors.New("couldn't handle this token")
)

var jwtConfig *config.JWTConfig

func StartJWT(cfg *config.Config) {
	jwtConfig = &cfg.JWTConfig
	if err := initJWT(); err != nil {
		zap.L().Fatal("jwt initialization failed", zap.String("err", err.Error()))
		return
	}
}

func initJWT() error {
	if jwtConfig.JwtSecret == "" {
		return errors.New("jwt secret cannot be empty")
	}
	return nil
}

func GenerateToken(UserID uint64, emailPtr, phonePtr *string, state byte) (string, error) {
	email, phone := "", ""
	if emailPtr != nil {
		email = *emailPtr
	}
	if phonePtr != nil {
		phone = *phonePtr
	}
	claims := CustomClaims{
		UserID: UserID,
		Email:  email,
		Phone:  phone,
		State:  state,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour * time.Duration(jwtConfig.ExpireHours))),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    jwtConfig.Issuer,
		},
	}

	signMethod := jwt.SigningMethodHS256
	switch jwtConfig.SignMethod {
	case "hs256":
		signMethod = jwt.SigningMethodHS256
	case "hs384":
		signMethod = jwt.SigningMethodHS384
	case "hs512":
		signMethod = jwt.SigningMethodHS512
	}

	token := jwt.NewWithClaims(signMethod, claims)
	return token.SignedString([]byte(jwtConfig.JwtSecret))
}

func ParseToken(tokenStr string) (*CustomClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &CustomClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrTokenInvalid
		}
		return []byte(jwtConfig.JwtSecret), nil
	})
	// 错误处理细化
	if err != nil {
		if errors.Is(err, jwt.ErrTokenMalformed) {
			return nil, ErrTokenMalformed
		} else if errors.Is(err, jwt.ErrTokenExpired) || errors.Is(err, jwt.ErrTokenNotValidYet) {
			return nil, ErrTokenExpired
		} else {
			return nil, ErrTokenInvalid
		}
	}
	if claims, ok := token.Claims.(*CustomClaims); ok && token.Valid {
		return claims, nil
	}
	return nil, ErrTokenInvalid
}

// ExactToken 从请求头中提取token字符串
func ExactToken(authHeader string) string {
	if authHeader == "" {
		return ""
	}
	parts := strings.Split(authHeader, " ")
	if len(parts) == 2 && parts[0] == "Bearer" {
		return parts[1]
	}
	return ""
}
