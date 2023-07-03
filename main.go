package main

import (
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"imapApi/api"
	"io/ioutil"
	"log"
	"os"
	"time"
)

func main() {

	// Connect to SQLite database
	var err error
	newLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags), // io writer
		logger.Config{
			SlowThreshold: time.Second,   // 慢 SQL 阈值
			LogLevel:      logger.Silent, // Log level
			Colorful:      false,         // 禁用彩色打印
		},
	)
	api.Db, err = gorm.Open(sqlite.Open("mails.db"), &gorm.Config{
		Logger: newLogger,
	})
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}

	// Auto migrate the UserMails model
	if err := api.Db.AutoMigrate(&api.UserMails{}); err != nil {
		log.Fatalf("failed to create table: %v", err)
	}

	// 设置api路由
	gin.SetMode(gin.ReleaseMode)
	//todo  关闭gin和gorm的日志
	gin.DefaultWriter = ioutil.Discard

	r := gin.Default()
	r.GET("/mail/Login", api.Login)
	r.GET("/mail/startGetMail", api.StartGetMail)
	r.GET("/mail/getMail", api.GetMail)
	r.GET("/mail/getMailWait", api.GetMailWait)
	r.GET("/mail/logout", api.Logout)

	err = r.Run(":8080")
	if err != nil {
		log.Fatalf("failed to run server: %v", err)
	}
}
