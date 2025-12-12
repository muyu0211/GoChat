package handler

import (
	"GoChat/internel/model/dto"
	"GoChat/internel/service"
	"GoChat/pkg/util"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"net/http"
	"strings"
)

type UserHandler struct {
	userService service.IUserService
}

func NewUserHandler(userService service.IUserService) *UserHandler {
	return &UserHandler{userService: userService}
}

func (uh *UserHandler) SendVerifyCode(ctx *gin.Context) {
	var req dto.SendCodeRequest

	if err := ctx.ShouldBind(&req); err != nil {
		zap.L().Warn(fmt.Sprintf("参数错误: %v", err.Error()),
			zap.String("request_email_or_phone", req.EmailOrPhone))
		ctx.JSON(http.StatusBadRequest, util.NewResMsg("0", fmt.Sprintf("参数错误: %v", err.Error()), nil))
		return
	}

	// 格式验证是邮箱还是手机
	if strings.TrimSpace(req.EmailOrPhone) == "" {
		ctx.JSON(http.StatusBadRequest, util.NewResMsg("0", fmt.Sprintf("邮箱/手机号不能为空"), nil))
		return
	}

	if util.ValidEmail(req.EmailOrPhone) {
		if err := uh.userService.SendEmailCode(ctx.Request.Context(), "验证码", req.EmailOrPhone); err != nil {
			ctx.JSON(http.StatusInternalServerError, util.NewResMsg("0", fmt.Sprintf("服务繁忙, 请稍后重试"), nil))
			return
		}
		ctx.JSON(http.StatusOK, util.NewResMsg("0", fmt.Sprintf("验证码发送成功"), nil))
	} else if util.ValidPhone(req.EmailOrPhone) {
		if err := uh.userService.SendPhoneCode(ctx.Request.Context(), req.EmailOrPhone); err != nil {
			ctx.JSON(http.StatusInternalServerError, util.NewResMsg("0", fmt.Sprintf("服务繁忙, 请稍后重试"), nil))
			return
		}
		ctx.JSON(http.StatusOK, util.NewResMsg("0", fmt.Sprintf("验证码发送成功"), nil))
	} else {
		ctx.JSON(http.StatusInternalServerError, util.NewResMsg("0", fmt.Sprintf("邮箱/手机号格式错误"), nil))
		return
	}
}

func (uh *UserHandler) LoginInCode(ctx *gin.Context) {
	var req dto.LoginRequest
	if err := ctx.ShouldBind(&req); err != nil {
		ctx.Set("err", err)
		ctx.JSON(http.StatusInternalServerError, util.NewResMsg("0", "服务器繁忙，请稍后重试", nil))
		return
	}

	resp, err := uh.userService.LoginInCode(ctx, &req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidCredentials):
			ctx.JSON(http.StatusForbidden, util.NewResMsg("0", "邮箱/手机号格式错误", nil))
			return
		case errors.Is(err, service.ErrUserNotFound):
			ctx.JSON(http.StatusForbidden, util.NewResMsg("0", "账号不存在", nil))
			return
		case errors.Is(err, service.ErrUserDisabled):
			ctx.JSON(http.StatusForbidden, util.NewResMsg("0", "账号已被禁用", nil))
			return
		case errors.Is(err, service.ErrCodeError):
			ctx.JSON(http.StatusForbidden, util.NewResMsg("0", service.ErrCodeError.Error(), nil))
			return
		default:
			ctx.JSON(http.StatusInternalServerError, util.NewResMsg("0", "服务器繁忙, 请稍后重试", nil))
			return
		}
	}

	ctx.JSON(http.StatusOK, util.NewResMsg("1", "登陆成功", resp))
}

func (uh *UserHandler) LoginInPW(ctx *gin.Context) {
	var req dto.LoginRequest
	if err := ctx.ShouldBind(&req); err != nil {
		ctx.Set("err", err)
		ctx.JSON(http.StatusInternalServerError, util.NewResMsg("0", "服务器繁忙，请稍后重试", nil))
		return
	}

	resp, err := uh.userService.LoginInPW(ctx, &req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidCredentials):
			ctx.JSON(http.StatusForbidden, util.NewResMsg("0", "邮箱/手机号格式错误", nil))
			return
		case errors.Is(err, service.ErrUserNotFound):
			ctx.JSON(http.StatusForbidden, util.NewResMsg("0", "账号不存在", nil))
			return
		case errors.Is(err, service.ErrUserDisabled):
			ctx.JSON(http.StatusForbidden, util.NewResMsg("0", "账号已被禁用", nil))
			return
		case errors.Is(err, service.ErrCodeError):
			ctx.JSON(http.StatusForbidden, util.NewResMsg("0", service.ErrCodeError.Error(), nil))
			return
		default:
			ctx.JSON(http.StatusInternalServerError, util.NewResMsg("0", "服务器繁忙, 请稍后重试", nil))
			return
		}
	}

	ctx.JSON(http.StatusOK, util.NewResMsg("1", "登陆成功", resp))
}

func (uh *UserHandler) Register(ctx *gin.Context) {
	var req dto.RegisterRequest
	if err := ctx.ShouldBind(&req); err != nil {
		zap.L().Warn("参数绑定失败", zap.Error(err))
		ctx.JSON(http.StatusInternalServerError, util.NewResMsg("0", "服务器繁忙，请稍后重试", nil))
		return
	}

	if err := uh.userService.Register(ctx, &req); err != nil {
		if errors.Is(err, service.ErrServerNotAvailable) {
			ctx.JSON(http.StatusInternalServerError, util.NewResMsg("0", "服务器繁忙，请稍后重试", nil))
			return
		} else {
			ctx.JSON(http.StatusForbidden, util.NewResMsg("0", err.Error(), nil))
			return
		}

	}

	ctx.JSON(http.StatusOK, util.NewResMsg("1", "注册成功", nil))
}
