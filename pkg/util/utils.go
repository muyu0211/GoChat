package util

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"gopkg.in/gomail.v2"
	"hash/fnv"
	"math/rand"
	"regexp"
	"strings"
	"sync"
	"time"
)

func InitConfig() {
	// 使用viper读取配置文件，进行项目初始化
	viper.SetConfigFile("./config/config.yml")
	if err := viper.ReadInConfig(); err != nil {
		panic(fmt.Errorf("fatal error config file: %w", err))
	}
}

type ResMsg struct {
	Code string      `json:"code"`
	Msg  string      `json:"message"`
	Data interface{} `json:"data"`
}

type pair struct {
	Email string
	Err   error
}

func NewResMsg(Code, Msg string, Data interface{}) ResMsg {
	return ResMsg{
		Code: Code,
		Msg:  Msg,
		Data: Data,
	}
}

// ValidEmail 验证邮箱格式
func ValidEmail(email string) bool {
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	return emailRegex.MatchString(email)
}

// ValidPhone 验证手机号格式
func ValidPhone(telephone string) bool {
	zap.L().Warn("ValidPhone is not implemented")
	//phoneRegex := regexp.MustCompile(``)
	//return phoneRegex.MatchString(telephone)
	return false
}

// GenVerificationCode 生成指定长度的数字验证码
func GenVerificationCode(length int) string {
	seed := rand.New(rand.NewSource(time.Now().UnixNano()))
	var code string
	for i := 0; i < length; i++ {
		code += fmt.Sprintf("%d", seed.Intn(10))
	}
	return code
}

// SendEmail 发送邮件
func SendEmail(ctx context.Context, mailTmp string, emailTargetAddress ...string) <-chan pair {
	if len(emailTargetAddress) == 0 {
		errChan := make(chan pair, 1)
		defer close(errChan)
		errChan <- pair{
			Email: "",
			Err:   fmt.Errorf("邮件发送失败: 未选择邮件收件人"),
		}
		return errChan
	}

	// 使用协程进行批量发送邮件
	errChan := make(chan pair, len(emailTargetAddress))
	var wg sync.WaitGroup
	for _, emailAdd := range emailTargetAddress {
		recipient := emailAdd
		wg.Add(1)
		go func(recipient string) {
			defer wg.Done()

			// 创建STMP客户端
			host := EmailHost
			port := EmailPort
			username := EmailUser
			password := EmailPwd
			// 创建邮件
			m := gomail.NewMessage()
			m.SetHeader("From", username)
			m.SetHeader("To", recipient)
			m.SetHeader("Subject", EmailTitle)
			m.SetBody("text/html", mailTmp)

			// TODO:开发环境关闭TLS认证
			dial := gomail.NewDialer(host, port, username, password)
			dial.TLSConfig = &tls.Config{InsecureSkipVerify: true}

			for attempt := 0; attempt < CodeMaxSendNum; attempt++ {
				select {
				case <-ctx.Done():
					errChan <- pair{
						Email: recipient,
						Err:   fmt.Errorf("邮件发送失败: to: %v, err: %v\n", recipient, ctx.Err()),
					}
					return
				default:
				}

				if err := dial.DialAndSend(m); err != nil {
					if strings.Contains(err.Error(), "550") {
						errChan <- pair{
							Email: recipient,
							Err:   nil,
						}
						return
					}

					// 继续尝试发送邮件
					if attempt < CodeMaxSendNum-1 {
						select {
						case <-ctx.Done():
							errChan <- pair{
								Email: recipient,
								Err:   fmt.Errorf("邮件发送失败: %v: %v", recipient, ctx.Err()),
							}
							return
						case <-time.After(time.Millisecond * 500): // 延迟500ms后继续发送邮件
						}
					}
				} else {
					// 邮件发送成功
					errChan <- pair{
						Email: recipient,
						Err:   nil,
					}
					return
				}
			}
		}(recipient)
	}

	// 等待所有邮件发送完毕
	go func() {
		wg.Wait()
		close(errChan)
	}()
	return errChan
}

func IsWebsocket(c *gin.Context) bool {
	if !strings.EqualFold(c.Request.Header.Get("Upgrade"), "websocket") {
		return false
	}

	if !strings.Contains(strings.ToLower(c.Request.Header.Get("Connection")), "upgrade") {
		return false
	}

	if c.Request.Header.Get("Sec-WebSocket-Key") == "" {
		return false
	}

	return true
}

func IsGRPC(c *gin.Context) bool {
	contentType := c.Request.Header.Get("Content-Type")
	return strings.HasPrefix(strings.ToLower(contentType), "application/grpc")
}

func IsHTTP(c *gin.Context) bool {
	return !IsWebsocket(c) && !IsGRPC(c)
}

// GetHashed 获取fnv哈希算法的哈希值
func GetHashed(key string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(key))
	return h.Sum32()
}

// GetConversationID 生成会话ID的算法：按字典序排序，保证 A->B 和 B->A 是同一个会话
func GetConversationID(userA, userB uint64) string {
	if userA < userB {
		return fmt.Sprintf("%d_%d", userA, userB)
	}
	return fmt.Sprintf("%d_%d", userB, userA)
}
