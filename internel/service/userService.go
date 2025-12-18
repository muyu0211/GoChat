package service

import (
	"GoChat/internel/model/dao"
	"GoChat/internel/model/dto"
	"GoChat/internel/repository"
	"GoChat/internel/repository/cache"
	"GoChat/pkg/auth"
	"GoChat/pkg/util"
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"html/template"
	"strconv"
	"time"
)

type IUserService interface {
	CreateUser(context.Context, *dao.UserBasicModel) error
	SendEmailCode(context.Context, string, string) error
	SendPhoneCode(context.Context, string) error
	LoginInCode(context.Context, *dto.LoginRequest) (*dto.LoginResponse, error)
	LoginInPW(context.Context, *dto.LoginRequest) (*dto.LoginResponse, error)
	Register(context.Context, *dto.RegisterRequest) error
	UserOnline(ctx context.Context, userID uint64) error
	UserOffline(ctx context.Context, userID uint64) error
	GetUserLocation(ctx context.Context, userID uint64) (string, error)
}

const (
	_ = iota
	email
	phone
)

type userService struct {
	userRepo   repository.IUserRepo
	redisCache cache.ICacheRepository
	tx         ITxManager
}

func NewUserService(repo repository.IUserRepo, cache cache.ICacheRepository, tx ITxManager) IUserService {
	return &userService{userRepo: repo, redisCache: cache, tx: tx}
}

func (us *userService) CreateUser(ctx context.Context, user *dao.UserBasicModel) error {
	return nil
}

func (us *userService) SendEmailCode(ctx context.Context, title, email string) error {
	// 解析邮件模板
	tmp, err := template.ParseFiles("web/template/email.html")
	if err != nil {
		zap.L().Warn("mail.html not found in path",
			zap.Error(err))
		return errors.New(util.ErrService)
	}

	// 生成验证码
	code := util.GenVerificationCode(util.VerificationCodeLength)

	// 将验证码存入redis
	key := util.KeyVerifyCode + email
	if err = us.redisCache.Set(ctx, key, code, util.CodeExpireTime); err != nil {
		return err
	}

	// 构造邮件页面的标题和验证码
	emailContent := gin.H{
		"title": title,
		"code":  code,
	}
	execTmp := bytes.Buffer{}
	err = tmp.Execute(&execTmp, emailContent)
	if err != nil {
		zap.L().Warn(fmt.Sprintf("mail.html can not be executed: %v", err))
		return errors.New(util.ErrService)
	}
	// 转化成字符串
	mailTmp := execTmp.String()

	// 发送邮件, 设置10秒的有效时间
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	errChan := util.SendEmail(ctx, mailTmp, email)
	select {
	case errPair := <-errChan:
		if errPair.Err != nil {
			zap.L().Warn(fmt.Sprintf("Failed to send email (select) to %s: %v", errPair.Email, errPair.Err))
			return fmt.Errorf("failed to send email (select) to %s: %v", errPair.Email, errPair.Err)
		}
	case <-time.After(time.Second * 10):
		zap.L().Warn("Timed out waiting for email send result channel.")
		return errors.New("timed out waiting for email send result channel")
	}
	return nil
}

func (us *userService) SendPhoneCode(ctx context.Context, phone string) error {
	zap.L().Warn("SendPhoneCode is not implement")
	return errors.New("SendPhoneCode is not implement")
}

func checkCode(originCode, code string, err error) error {
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return ErrCodeError
		} else {
			zap.L().Warn("GetFromKey 错误", zap.Error(err))
			return fmt.Errorf("验证码校验失败，请稍后重试")
		}
	}
	if originCode != code {
		return ErrCodeError
	}
	return nil
}

// LoginInCode 验证码登录
func (us *userService) LoginInCode(ctx context.Context, req *dto.LoginRequest) (*dto.LoginResponse, error) {
	// 1. 验证邮箱/手机号
	flag := -1
	if util.ValidEmail(req.EmailOrPhone) {
		flag = email
	}
	if flag == -1 && util.ValidPhone(req.EmailOrPhone) {
		flag = phone
	}
	if flag == -1 {
		return nil, ErrInvalidCredentials
	}

	// 2. 从redis中获取验证码
	key := util.KeyVerifyCode + req.EmailOrPhone
	var code string
	ok, err := us.redisCache.Get(ctx, key, &code)
	if err != nil {
		return nil, ErrServerNotAvailable
	}
	if !ok {
		return nil, ErrCodeError
	}

	// 3. 判断验证码是否正确
	if code != req.VerifyCode {
		return nil, ErrCodeError
	}

	// 4. 验证码正确后查找数据库，返回用户数据
	var user *dao.UserBasicModel
	if flag == email {
		user, err = us.userRepo.GetByEmail(ctx, req.EmailOrPhone)
	}
	if flag == phone {
		user, err = us.userRepo.GetByPhone(ctx, req.EmailOrPhone)
	}

	if err != nil {
		return nil, errors.New("数据库炸了") // 数据库炸了
	}

	// 用户不存在
	if user == nil {
		return nil, ErrUserNotFound
	}

	// 5. 进行用户权限等判断
	if user.State == dao.UserStateBanned || user.State == dao.UserStateDeleted {
		return nil, ErrUserDisabled
	}

	// 6. 生成token
	token, err := auth.GenerateToken(uint64(user.ID), user.Email, user.Phone, string(user.State))
	if err != nil {
		zap.L().Error("failed to generate token", zap.Error(err))
		return nil, err
	}

	// 6. 构建并返回成功的响应
	resp := &dto.LoginResponse{
		Token:    token,
		ID:       user.ID,
		Username: user.Nickname,
	}
	if user.Email != nil {
		resp.Email = *user.Email
	}
	if user.Phone != nil {
		resp.Telephone = *user.Phone
	}

	return resp, nil
}

func (us *userService) LoginInPW(ctx context.Context, req *dto.LoginRequest) (*dto.LoginResponse, error) {
	// 1. 验证邮箱/手机号
	flag := -1
	if util.ValidEmail(req.EmailOrPhone) {
		flag = email
	}
	if flag == -1 && util.ValidPhone(req.EmailOrPhone) {
		flag = phone
	}
	if flag == -1 {
		return nil, ErrInvalidCredentials
	}

	// 2. 查找数据库，返回用户数据
	var user *dao.UserBasicModel
	var err error
	if flag == email {
		user, err = us.userRepo.GetByEmail(ctx, req.EmailOrPhone)
	}
	if flag == phone {
		user, err = us.userRepo.GetByPhone(ctx, req.EmailOrPhone)
	}

	if err != nil {
		return nil, errors.New("数据库炸了") // 数据库炸了
	}
	if user == nil {
		return nil, ErrUserNotFound
	}

	// 进行密码校验
	if req.Password != "" && !(req.Password == user.PassWordHashed) {
		return nil, ErrPWError
	}

	// 5. 进行用户权限等判断
	if user.State == dao.UserStateBanned || user.State == dao.UserStateDeleted {
		return nil, ErrUserDisabled
	}

	// 6. 生成token
	token, err := auth.GenerateToken(user.ID, user.Email, user.Phone, string(user.State))
	if err != nil {
		zap.L().Error("failed to generate token", zap.Error(err))
		return nil, err
	}

	// 6. 构建并返回成功的响应
	resp := &dto.LoginResponse{
		Token:    token,
		ID:       user.ID,
		Username: user.Nickname,
	}
	if user.Email != nil {
		resp.Email = *user.Email
	}
	if user.Phone != nil {
		resp.Telephone = *user.Phone
	}

	return resp, nil
}

func (us *userService) Register(ctx context.Context, req *dto.RegisterRequest) error {
	// 1. 验证邮箱/手机号
	flag := -1
	if util.ValidEmail(req.EmailOrPhone) {
		flag = email
	}
	if flag == -1 && util.ValidPhone(req.EmailOrPhone) {
		flag = phone
	}
	if flag == -1 {
		return ErrInvalidCredentials
	}

	// 2. 从redis中获取验证码
	key := util.KeyVerifyCode + req.EmailOrPhone
	var code string
	ok, err := us.redisCache.Get(ctx, key, &code)
	if err != nil {
		return ErrServerNotAvailable
	}
	if !ok {
		return ErrCodeError
	}

	// 3. 创建新用户
	user := &dao.UserBasicModel{
		Nickname:       req.Nickname,
		PassWordHashed: req.Password,
		State:          1,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	if flag == email {
		user.Email = &req.EmailOrPhone
	} else {
		user.Phone = &req.EmailOrPhone
	}
	err = us.userRepo.Create(ctx, user)
	if err != nil {
		zap.L().Warn("用户创建失败", zap.Error(err))
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return ErrUserAlreadyExist
		} else {
			return ErrServerNotAvailable
		}
	}

	return nil
}

// UserOnline 用户上线：记录 用户：UserID 所在的服务器结点的ID： ServerID
func (us *userService) UserOnline(ctx context.Context, userID uint64) error {
	key := fmt.Sprintf("%s:%s", util.KeyUserLocation, strconv.FormatUint(userID, 10))
	err := us.redisCache.Set(ctx, key, util.ServerID, util.CodeUserLocationExpire)
	if err != nil {
		return ErrServerNotAvailable
	}
	return nil
}

// UserOffline 用户离线：删除记录
func (us *userService) UserOffline(ctx context.Context, userID uint64) error {
	key := fmt.Sprintf("%s:%s", util.KeyUserLocation, strconv.FormatUint(userID, 10))
	// 严谨做法：这里应该用 Lua 脚本校验 Value 是否是当前 ServerID 再删
	// 防止并发下误删了用户刚重连到另一台服务器的状态
	// 这里简化为直接删
	if err := us.redisCache.Delete(ctx, key); err != nil {
		return ErrServerNotAvailable
	}
	return nil
}

func (us *userService) GetUserLocation(ctx context.Context, userID uint64) (string, error) {
	key := fmt.Sprintf("%s:%s", util.KeyUserLocation, strconv.FormatUint(userID, 10))
	var location string
	ok, err := us.redisCache.Get(ctx, key, &location)
	if err != nil {
		zap.L().Error("failed to get user location", zap.Error(err))
		return "", ErrServerNotAvailable
	}
	if !ok {
		return "", redis.Nil
	}
	return location, nil
}
