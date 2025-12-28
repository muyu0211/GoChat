package main

import (
	"GoChat/internel/repository"

	"github.com/google/wire"
)

// 定义 db 集合

// 定义 infrastructure 集合

// 定义 repository 集合
var repositorySet = wire.NewSet(
	repository.NewUserRepo,
	repository.NewChatRepo,
	repository.NewConvRepo,
)

// 定义 service 集合

// 定义 handler 集合
