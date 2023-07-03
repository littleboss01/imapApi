package api

import (
	"github.com/gin-gonic/gin"
	"golang.org/x/net/context"
	"gorm.io/gorm"
	"log"
	"strconv"
	"sync"
	"time"
)

type UserMails struct {
	//gorm.Model
	Username   string
	Title      string
	To         string
	Content    string
	Expiration time.Time
	Regex      string
	IsDel      bool
}

var Db *gorm.DB
var Mails = make(map[string]*Imap)

var mutex_mail = &sync.Mutex{}
var mutex_sql = &sync.Mutex{}

// 安全地取出数据
func SafeGet(key string) *Imap {
	mutex_mail.Lock()
	defer mutex_mail.Unlock()

	// 取出数据
	imap, ok := Mails[key]
	if !ok {
		return nil
	}
	return imap
}

// 删除imap
func DelImap(key string) {
	mutex_mail.Lock()
	defer mutex_mail.Unlock()

	delete(Mails, key)
}

// 安全地插入数据
func SafeSet(key string, imap *Imap) {
	mutex_mail.Lock()
	defer mutex_mail.Unlock()

	Mails[key] = imap
}
func Login(c *gin.Context) {
	user := c.Query("user")
	pass := c.Query("pass")
	var isDelAll = false
	var err error
	isDelAll, err = strconv.ParseBool(c.Query("isDelAll")) //isDelStr转换为bool
	if err != nil {
		c.JSON(200, gin.H{"code": -1, "msg": "isDelAll 转换为 bool 失败"})
		return
	}
	code := 0
	var msg string
	//如果user已经登录,则返回登录成功
	imap := SafeGet(user)
	if imap == nil { //todo 同时插入会出现资源竞争
		SafeSet(user, &Imap{})
		imap = SafeGet(user)
		code = imap.Login(3, user, pass, isDelAll)
	} else {
		code = 1
	}
	if code == 1 {
		_, err2 := imap.Cli.Select("INBOX", false)
		if err2 != nil {
			log.Println("select", err2)
		}
		//Mails[user].Cli.SetDebug(os.Stdout)
		code = 1
		msg = "ok"
	} else {
		msg = "登录失败"
	}
	c.JSON(200, gin.H{
		"code": code,
		"msg":  msg,
	})
}

func StartGetMail(c *gin.Context) {
	user := c.Query("user")
	title := c.Query("title")
	to := c.Query("to")
	startTime := c.Query("startTime")
	regex := c.Query("regex")
	timeOUt, _ := strconv.Atoi(c.Query("timeOUt"))
	if timeOUt == 0 {
		timeOUt = 30
	}
	isDel, err := strconv.ParseBool(c.Query("isDel")) //isDelStr转换为bool
	if err != nil {
		c.JSON(200, gin.H{"code": -1, "msg": "isDelStr 转换为 bool 失败"})
		return
	}
	imap := SafeGet(user)
	if imap == nil {
		c.JSON(200, gin.H{"code": -1, "msg": "邮箱 未登录"})
		return
	}
	var ctx context.Context
	var cancelFunc context.CancelFunc
	ctx, cancelFunc = context.WithTimeout(context.Background(), time.Duration(timeOUt)*time.Second)
	go func() {
		defer cancelFunc() //确保在匿名函数执行完毕前，运行 cancelFunc 函数
		var msg string
		var err1 error
		_, err2 := imap.Cli.Select("INBOX", false)
		if err2 != nil {
			log.Println("select", err2)
		}
		for {
			/*select {

			case <-time.After(3 * time.Second):
				log.Println("收取邮件中...")

				msg, err1 = imap.GetMailByTitleAndTime(title, to, startTime, regex, isDel)
				if err1 != nil {
					msg = "邮件收取失败"
					return
				}
				if msg != "" {
					log.Println("msg:", msg)
					now := time.Now()
					// Add message to database with expiration time
					userMail := &UserMails{Username: user, Title: title, To: to, Content: msg, Expiration: now.Add(60 * time.Second)}

					mutex_sql.Lock()
					Db.Create(userMail)
					mutex_mail.Unlock()

					return
				}
			case <-ctx.Done():
				msg = "邮件收取超时"
				log.Println("邮件收取超时")
				return

			}*/
			log.Println("收取邮件中...")

			msg, err1 = imap.GetMailByTitleAndTime(title, to, startTime, regex, isDel)
			if err1 != nil {
				msg = "邮件收取失败"
				return
			}
			if msg != "" {
				log.Println("msg:", msg)
				now := time.Now()
				// Add message to database with expiration time
				userMail := &UserMails{Username: user, Title: title, To: to, Content: msg, Expiration: now.Add(60 * time.Second)}

				mutex_sql.Lock()
				Db.Create(userMail)
				mutex_mail.Unlock()
			}
			time.Sleep(3 * time.Second)
			if ctx.Err() == context.DeadlineExceeded {
				msg = "邮件收取超时"
				log.Println("邮件收取超时")
				break
			}

		}
	}()

	c.JSON(200, gin.H{"status": "mail fetching started"})
}
func GetMailWait(c *gin.Context) {
	user := c.Query("user")
	pass := c.Query("pass")
	title := c.Query("title")
	to := c.Query("to")
	startTime := c.Query("startTime")
	regex := c.Query("regex")
	timeOUt, _ := strconv.Atoi(c.Query("timeOUt"))
	if timeOUt == 0 {
		timeOUt = 30
	}
	isDel, err := strconv.ParseBool(c.Query("isDel")) //isDelStr转换为bool
	if err != nil {
		c.JSON(200, gin.H{"code": -1, "msg": "isDelStr 转换为 bool 失败"})
		return
	}
	imap := &Imap{}
	if imap.Login(3, user, pass, false) == 1 {
		_, err2 := imap.Cli.Select("INBOX", false)
		if err2 != nil {
			log.Println("select", err2)
		}
		var msg string
		var err1 error
		var ctx context.Context
		var cancelFunc context.CancelFunc
		ctx, cancelFunc = context.WithTimeout(context.Background(), time.Duration(timeOUt)*time.Second)
		go func() {
			defer cancelFunc() //确保在匿名函数执行完毕前，运行 cancelFunc 函数
			for {
				log.Println("收取邮件中...")

				msg, err1 = imap.GetMailByTitleAndTime(title, to, startTime, regex, isDel)
				if err1 != nil {
					msg = "邮件收取失败"
					break
				}
				if msg != "" {
					log.Println("msg:", msg)
					break
				}
				if ctx.Err() == context.DeadlineExceeded {
					msg = "邮件收取超时"
					log.Println("邮件收取超时")
					break
				}
				time.Sleep(3 * time.Second)
			}

		}()
		c.JSON(200, gin.H{"status": "mail fetching started"})
	} else {
		c.JSON(200, gin.H{"status": "mail fetching failed"})
	}
}
func GetMail(c *gin.Context) {
	var userMail UserMails
	userMail.Username = c.Query("user")
	userMail.Title = c.Query("title")
	userMail.To = c.Query("to")

	mutex_mail.Lock()
	result := Db.Where(&userMail).First(&userMail)
	mutex_mail.Unlock()

	if result.Error != nil {
		c.JSON(200, gin.H{"msg": "邮件未收取"})
		return
	}

	if userMail.Expiration.Before(time.Now()) { //判断邮件的时间是否小于当前时间
		c.JSON(200, gin.H{"msg": "邮件已过期"})
		return
	}
	{
		mutex_mail.Lock()
		Db.Unscoped().Where(&userMail).Delete(&userMail) //todo 删除数据库中的数据,为什么不是硬删除
		mutex_mail.Unlock()
	}
	// Return message content
	c.JSON(200, gin.H{"code": 1, "msg": userMail.Content})
}
func Logout(c *gin.Context) {
	user := c.Query("user")
	//isAll_str := c.Query("isAll")
	//if isAll_str != "" {
	//	isAll, err := strconv.ParseBool(isAll_str) //isAll_str转换为bool
	//	if err != nil {
	//		log.Println("isAll_str 转换为 bool 失败",err)
	//		c.JSON(200, gin.H{"code": -1, "msg": "isAll_str 转换为 bool 失败"})
	//		return
	//	}
	//	if isAll {
	//		Mails = nil
	//	}
	//}

	imap := SafeGet(user)
	if imap != nil {
		err := imap.Cli.Logout()
		if err != nil {
			log.Println("logout err:", err)
		}
		//从map中删除
		DelImap(user)
		c.JSON(200, gin.H{
			"msg": "logout",
		})
	}

}
