package api

import (
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"net"
	"net/smtp"
)

type loginAuth struct {
	username, password string
}

func LoginAuth(username, password string) smtp.Auth {
	return &loginAuth{username, password}
}

func (a *loginAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	return "LOGIN", []byte(a.username), nil
}

func (a *loginAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if more {
		switch string(fromServer) {
		case "Username:":
			return []byte(a.username), nil
		case "Password:":
			return []byte(a.password), nil
		default:
			return nil, errors.New("Unknown from server")
		}
	}
	return nil, nil
}
func SendMail(c *gin.Context) {
	user := c.Query("user")
	pass := c.Query("pass")
	to := c.Query("to")
	subject := c.Query("subject")
	body := c.Query("body")
	err := Send(user, pass, to, subject, body)
	if err != nil {
		fmt.Println(err)
	}
}
func Send(user, pass, to, subject, body string) error {
	smtpHost := "smtp.office365.com"
	//smtpPort := "587"

	conn, err := net.Dial("tcp", "smtp.office365.com:587")
	if err != nil {
		println(err)
	}

	c, err := smtp.NewClient(conn, smtpHost)
	if err != nil {
		println(err)
	}

	tlsconfig := &tls.Config{
		ServerName: smtpHost,
	}

	if err = c.StartTLS(tlsconfig); err != nil {
		println(err)
	}

	auth := LoginAuth(user, pass)

	if err = c.Auth(auth); err != nil {
		println(err)
	}

	var bytes bytes.Buffer

	mimeHeaders := "MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\n\n"
	bytes.Write([]byte(fmt.Sprintf("to:%s\nSubject: %s \n%s\n\n", to, subject, mimeHeaders)))

	// Sending email.
	/*	err = smtp.SendMail(smtpHost+":"+smtpPort, auth, user, to, body.Bytes())
		if err != nil {
			fmt.Println(err)
			return
		}*/

	err = c.Mail(user)
	if err != nil {
		fmt.Println(err)
		return err
	}

	err = c.Rcpt(to) // 发件人地址作为唯一的收件人
	if err != nil {
		fmt.Println(err)
		return err
	}

	w, err := c.Data()
	if err != nil {
		fmt.Println(err)
		return err
	}

	_, err = w.Write(bytes.Bytes())
	if err != nil {
		fmt.Println(err)
		return err
	}

	err = w.Close()
	if err != nil {
		fmt.Println(err)
		return err
		fmt.Println("Email Sent!")
	}
	return nil
}
