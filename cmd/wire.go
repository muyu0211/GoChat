//go:build wireinject
// +build wireinject

package main

import (
	"GoChat/config"
	"GoChat/internel/handler"
	"GoChat/internel/infrastructure/mq"
	"GoChat/internel/repository"
	"GoChat/internel/repository/cache"
	"GoChat/internel/service"
	"GoChat/pkg/db"

	"github.com/go-redis/redis/v8"
	"github.com/google/wire"
	"gorm.io/gorm"
)

func providerConfig() *config.Config {
	return config.Cfg
}

func providerDBMaster() *gorm.DB {
	return db.GetDBS().Master
}

func providerRDB() *redis.Client {
	return db.GetRDS()
}

func providerKafkaACKProducer(cfg *config.Config) (*mq.AckProducer, error) {
	return mq.NewAckProducer(
		cfg.KafkaConfig.Brokers,
		&cfg.KafkaConfig.AckConfig,
	)
}

func providerKafkaACKConsumer(cfg *config.Config) (*mq.AckConsumer, error) {
	return mq.NewAckConsumer(
		cfg.KafkaConfig.Brokers,
		&cfg.KafkaConfig.AckConfig,
	)
}

// 定义 infrastructure 集合
var infrastructureSet = wire.NewSet(
	providerConfig,
	providerDBMaster,
	providerRDB,
	providerKafkaACKProducer,
	providerKafkaACKConsumer,
)

// 定义 repository 集合
var repositorySet = wire.NewSet(
	repository.NewUserRepo,
	repository.NewChatRepo,
	repository.NewConvRepo,
	repository.NewGroupRepo,
)

var cacheSet = wire.NewSet(cache.NewRedisCache)

// 定义 service 集合
var serviceSet = wire.NewSet(
	service.NewUserService,
	wire.Bind(new(service.IUserService), new(*service.UserService)),

	service.NewChatService,
	wire.Bind(new(service.IChatService), new(*service.ChatService)),

	service.NewGroupService,
	wire.Bind(new(service.IGroupService), new(*service.GroupService)),

	service.NewSyncService,
	wire.Bind(new(service.ISyncService), new(*service.SyncService)),

	service.NewPushService,
	wire.Bind(new(service.IPushService), new(*service.PushService)),

	service.NewTxManager,
	wire.Bind(new(service.ITxManager), new(*service.TxManager)),

	service.NewSeqFactory,
	wire.Bind(new(service.ISeqFactoryService), new(*service.SeqFactoryService)),
)

// 定义 handler 集合
var handlerSet = wire.NewSet(
	handler.NewUserHandler,
	handler.NewChatHandler,
	handler.NewGroupHandler,
)

func InitializeApp() (*App, error) {
	panic(wire.Build(
		infrastructureSet,
		repositorySet,
		cacheSet,
		serviceSet,
		handlerSet,
		wire.Struct(new(App), "*")))
}
