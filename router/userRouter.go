package router

import (
	"GoChat/internel/handler"
	"GoChat/internel/middleware"

	"github.com/gin-gonic/gin"
)

//func RegisterUserRouter(r *gin.Engine) {
//	dbs := db.GetDBS()
//	rds := db.GetRDS()
//	txm := service.NewTxManager(dbs.Master)
//
//	userRepo := repository.NewUserRepo(dbs.Master)
//	userCache := cache.NewRedisCache(rds)
//	userService := service.NewUserService(userRepo, userCache, txm)
//	userHandler := handler.NewUserHandler(userService)
//	r.POST("/verify_code", userHandler.SendVerifyCode)
//	r.POST("/login_email_code", userHandler.LoginInCode)
//	r.POST("/login_email_password", userHandler.LoginInPW)
//	r.POST("/register", userHandler.Register)
//
//	userApi := r.Group("/user")
//	userApi.Use(middleware.JWTMiddleware())
//}

func RegisterUserRouter(r *gin.Engine, userHandler *handler.UserHandler) {
	r.POST("/verify_code", userHandler.SendVerifyCode)
	r.POST("/login_email_code", userHandler.LoginInCode)
	r.POST("/login_email_password", userHandler.LoginInPW)
	r.POST("/register", userHandler.Register)

	userApi := r.Group("/user")
	userApi.Use(middleware.JWTMiddleware())
}
