package service

import (
	"GoChat/internel/model/dao"
	"GoChat/internel/model/dto"
	"context"
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
