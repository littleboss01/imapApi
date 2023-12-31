package api

import (
	"MyUitls"
	"crypto/tls"
	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-message/charset"
	"github.com/emersion/go-message/mail"
	"golang.org/x/net/context"
	"io"
	"io/ioutil"
	"log"
	"reflect"
	"regexp"
	"strings"
	"time"
)

type Imap struct {
	Cli      *client.Client
	username string
	password string
}

func (g *Imap) GetMail(api Api) (code int, msg string) {
	var err1 error
	//var id uint32
	var ctx context.Context
	var cancelFunc context.CancelFunc
	ctx, cancelFunc = context.WithTimeout(context.Background(), time.Duration(api.TimeOUt)*time.Second)

	defer cancelFunc() //确保在匿名函数执行完毕前，运行 cancelFunc 函数
	defer func() {
		go func() {
			g.Cli.Logout()
		}()
	}()
	boxName := ""
	for {
		if boxName != "INBOX" {
			boxName = "INBOX"
		} else { //垃圾箱
			boxName = "Junk"
		}
		if g.SelectMailBox(boxName) == 1 {
			log.Println("收取邮件中...")

			msg, err1, _ = g.GetMailByTitleAndTime(api.Title, api.To, api.StartTime, api.Regex, api.IsDel)
			if err1 != nil {
				msg = "邮件收取失败"
				break
			}
			if msg != "" {
				log.Println("msg:", msg)
				break
			}
		}

		if ctx.Err() == context.DeadlineExceeded {
			msg = "邮件收取超时"
			log.Println("邮件收取超时")
			break
		}
		time.Sleep(3 * time.Second)
	}
	return 1, msg
}

// 通过邮箱账号获取的gmail或者outlook的imap地址
func (g *Imap) GetImapAddr(email string) (mailAddr string, serverName string, isSSl bool) {
	if email == "" {
		return "", "", false
	}
	if strings.Contains(email, "gmail") {
		mailAddr = "imap.gmail.com:993"
		serverName = "imap.gmail.com"
		isSSl = true
	} else if strings.Contains(email, "outlook") || strings.Contains(email, "hotmail") {
		mailAddr = "outlook.office365.com:993"
		serverName = "outlook.office365.com"
		isSSl = true
	} else if strings.Contains(email, "21cn.com") {
		mailAddr = "imap-ent.21cn.com:993"
		serverName = "imap-ent.21cn.com"
		isSSl = true
	} else if strings.Contains(email, "pec.it") {
		mailAddr = "imaps.pec.aruba.it:993"
		serverName = "imaps.pec.aruba.it"
		isSSl = true
	} else if strings.Contains(email, "t-online.de") {
		mailAddr = "secureimap.t-online.de:993"
		serverName = "secureimap.t-online.de"
		isSSl = true
	} else if strings.Contains(email, "alice.it") {
		mailAddr = "in.alice.it:143"
		serverName = "in.alice.it"
		isSSl = false
	} else if strings.Contains(email, "sina.com") {
		mailAddr = "imap.sina.com:993"
		serverName = "imap.sina.com"
		isSSl = false
	} else {
		//取出邮箱@右边的字符
		mailAddr = strings.Split(email, "@")[1]
		mailAddr = MyUitls.DomainToIp(mailAddr)
		mailAddr = mailAddr + ":993"
		serverName = mailAddr
		isSSl = false
	}
	return mailAddr, serverName, isSSl
}

func (g *Imap) Login(count int, user string, pass string, isDelAll bool) int {
	var err error
	g.username = user
	g.password = pass
	mailAddr, ServerName, isSsl := g.GetImapAddr(user)
	for i := 0; i < count; i++ {
		if isSsl {
			g.Cli, err = client.DialTLS(mailAddr, &tls.Config{
				ServerName:         ServerName,
				InsecureSkipVerify: true,             // 验证服务器证书
				MinVersion:         tls.VersionTLS12, // 仅允许使用 TLS1.2 及以上的版本
			})
		} else {
			g.Cli, err = client.Dial(mailAddr)
		}

		if err != nil {
			log.Println("Unable to connect to IMAP server: ", err)
		}

		err = g.Cli.Login(user, pass)
		if err != nil {
			log.Println("Unable to login to IMAP server: ", err)
			if strings.Contains(err.Error(), "Invalid user name or password") {
				break
			}
		} else {
			/*go func(imapClient *client.Client) {
				for {
					if err := imapClient.Noop(); err != nil {
						log.Println("IMAP connection down, trying to reconnect...")
						newImapClient, err := client.DialTLS(mailAddr, &tls.Config{
							ServerName:         ServerName,
							InsecureSkipVerify: true,
							MinVersion:         tls.VersionTLS12,
						})
						if err != nil {
							log.Fatal("Unable to reconnect to IMAP server: ", err)
						}
						// Re-authenticate the client
						if err = newImapClient.Login(user, pass); err != nil {
							log.Fatal("Unable to re-authenticate to IMAP server: ", err)
						}
						g.cli = newImapClient
					}
					time.Sleep(10 * time.Second)
				}
			}(g.cli)*/
			if isDelAll {
				err = g.DelAllMail()
				if err != nil {
					log.Println(err)
				}
			}
			return 1
		}
		time.Sleep(time.Second * 1)
	}
	return 0
}

// 进入文件夹
func (g *Imap) SelectMailBox(mailBox string) int {
	if mailBox == "" {
		mailBox = "INBOX"
	}
	_, err := g.Cli.Select(mailBox, false)
	if err != nil {
		log.Println("select", err)
		return -1
	}
	return 1
}
func CallCount[T any](count int, stopValues []T, f func(args ...T) T, args ...T) T {
	var result T
	for i := 0; i < count; i++ {
		result = f(args...)
		for _, stopValue := range stopValues {
			if reflect.DeepEqual(result, stopValue) {
				break
			}
		}
	}
	return result
}

func (i *Imap) GetMailList() ([]uint32, error) {
	criteria := imap.NewSearchCriteria()
	ids, err := i.Cli.Search(criteria)
	if err != nil {
		return []uint32{}, err
	}
	return ids, nil
}

func (g *Imap) GetMailByCondition(title, to, startTime string) ([]uint32, error) {
	//todo 使用go-imap的时候 outlook不支持ascii编码 ，那怎么检索中文标题的邮件呢？
	criteria := imap.NewSearchCriteria()
	//criteria.WithoutFlags = []string{imap.SeenFlag} //排除已读邮件
	if title != "" {
		matched, _ := regexp.MatchString("[^\x00-\x7F]+", title)
		if matched {
			// Base64 encode the title and add it to the search criteria
			/*		gbkEncoder := simplifiedchinese.GBK.NewEncoder()
					bytes, _ := gbkEncoder.Bytes([]byte(title))*/
			// 进行 Base64 编码
			//bytes := []byte(title)
			//encodedTitle := base64.StdEncoding.EncodeToString(bytes)
			//criteria.Header.Set("Subject", encodedTitle)
			//titleWithWildcard := "*" + encodedTitle + "*"
			//criteria.Text = []string{"Subject", titleWithWildcard}
			//criteria.Header.Add("Subject", "=?utf-8?b?44CQ5ZOU5ZOp5ZOU5ZOp44CR6LSm5Y+35a6J5YWo5Lit5b+DLeaUueWvhg==?=")
			//criteria.Header.Add("Subject", "【哔哩哔哩】账号安全中心-改密操作提醒")

		} else {
			criteria.Header.Set("Subject", title)
		}
	}
	if to != "" {
		criteria.Header.Set("To", to)
	}
	//startTime 支持 2023-8-15 08:04:59吗

	if startTime != "" {
		criteria.Header.Set("Date", startTime)
	}
	ids, err := g.Cli.Search(criteria)
	if err != nil {
		log.Println("Search", err)
		//如果错误是因为连接断开了,则重新连接
		if strings.Contains(err.Error(), "connection is closed") ||
			strings.Contains(err.Error(), "connection was aborted") {
			//重新连接
			if g.Login(3, g.username, g.password, false) == 1 {
				g.Cli.Select("INBOX", false)
				ids, err = g.Cli.Search(criteria)
			}
		}
		return []uint32{}, err
	}
	/*	//ids按大到小排序,这里的排序无意义后面添加message的时候会升序
		sort.Slice(ids, func(i, j int) bool {
			return ids[i] > ids[j]
		}	)
		log.Println("ids", ids)*/
	return ids, nil
}

// 删除所有邮件
func (g *Imap) DelAllMail() error {
	var boxNames = []string{"INBOX", "Junk"}
	for _, boxName := range boxNames {
		_, err := g.Cli.Select(boxName, false)
		if err != nil {
			log.Println(err)
		}
		ids, err := g.GetMailList()
		if err != nil {
			return err
		}
		if len(ids) > 0 {
			g.DelMail(ids)
		}
	}
	return nil
}

// 删除邮件
func (g *Imap) DelMail(ids []uint32) error {
	seqset := new(imap.SeqSet)
	seqset.AddNum(ids...)
	//标记删除
	item := imap.FormatFlagsOp(imap.AddFlags, true)
	flags := []interface{}{imap.DeletedFlag}
	if err := g.Cli.Store(seqset, item, flags, nil); err != nil {
		return err
	}
	//执行删除
	if err := g.Cli.Expunge(nil); err != nil {
		return err
	}
	return nil
}

// 根据ids收取邮件
func (g *Imap) GetMailByIds(ids []uint32) ([]*imap.Message, error) {
	seqset := new(imap.SeqSet)
	//获取的时候不标记已读
	section := &imap.BodySectionName{
		Peek: true,
	}

	seqset.AddNum(ids...)                        //这里会从小到大排序
	item := []imap.FetchItem{imap.FetchEnvelope, //FetchEnvelope是邮件的头部信息
		imap.FetchInternalDate, //FetchInternalDate是邮件的时间
		imap.FetchRFC822,       //FetchRFC822是邮件的内容
		imap.FetchFlags,        //FetchFlags是邮件的标记
		section.FetchItem(),
	}
	messages := make(chan *imap.Message, 100)
	done := make(chan error, 1)
	go func() {
		done <- g.Cli.Fetch(seqset, item, messages)
	}()

	// Wait for fetch to be completed
	if err := <-done; err != nil {
		return nil, err
	}

	var msgList = make([]*imap.Message, len(ids))
	for i := len(messages) - 1; i >= 0; i-- {
		msgList[i] = <-messages
	}
	/*	for msg := range messages {
			msgList = append(msgList, msg)
		}
		sort.Slice(msgList, func(i, j int) bool {
			return msgList[i].SeqNum > msgList[j].SeqNum
		})*/
	return msgList, nil
}

// GetMailByTitleAndTime 获取包含指定标题,指定收件人,指定时间范围内的邮件 retrieves the first email message matching the given criteria
func (g *Imap) GetMailByTitleAndTime(title, to, startTime, regex string, isDel bool) (info string, err error, id uint32) {

	ids, _ := g.GetMailByCondition(title, to, startTime)
	if len(ids) == 0 {
		return "", nil, 0
	}

	messages, _ := g.GetMailByIds(ids)
	if len(messages) == 0 {
		return "", nil, 0
	}
	var msg *imap.Message
	for _, v := range messages {
		if strings.Contains(v.Envelope.Subject, title) {
			msg = v
			log.Println(v.Envelope.Date.String())
			id = v.Uid
			break
		}
	}

	bodySection, _ := imap.ParseBodySectionName("BODY[]")
	bodySection.Peek = true //不标记已读
	data := msg.GetBody(bodySection)
	//

	mr, err := mail.CreateReader(data)
	if err != nil {
		log.Println("CreateReader", err)
	}

	// 按顺序读取邮件的各个部分
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		} else if err != nil {
			log.Println("NextPart", err)
		}

		switch h := p.Header.(type) {
		case *mail.InlineHeader:
			if strings.HasPrefix(h.Get("Content-Type"), "text/plain") ||
				strings.HasPrefix(h.Get("Content-Type"), "text/html") {
				// 识别和解析邮件正文
				body, err := ioutil.ReadAll(p.Body)
				if err != nil {
					// 处理错误
					// 将字符集解码为UTF-8
					decoder, err := charset.Reader(h.Get("Content-Type"), p.Body)
					if err != nil {
						// 处理错误
					}
					// 读取解码后的字符串
					body, err = ioutil.ReadAll(decoder)
					if err != nil {
						// 处理错误
					}
				}
				info = info + string(body)
			}
		case *mail.AttachmentHeader:
			// 识别和处理附件
			//filename, _ := h.Filename()
			//log.Println("Got attachment: %v", filename)
		}
	}

	//log.Println("Email body text:", info)

	if regex != "" && msg != nil {
		re := regexp.MustCompile(regex)
		match := re.FindStringSubmatch(info)

		// 提取正则表达式匹配到的信息
		if len(match) == 0 {
			// 如果匹配不到任何信息，直接存储

		} else if len(match) > 0 {
			// 否则，将匹配到的信息拼接成一个字符串
			//info = strings.Join(match[0:], " | ")
			//0是整个匹配到的字符串，1是第一个括号匹配到的字符串
			info = match[1]
		}
	}
	if isDel {
		err = g.DelMail([]uint32{id})
		if err != nil {
			log.Println(err)
		}
	}
	//info之前加上邮件的时间
	//info = msg.Envelope.Date.String() + "\n" + info
	return info, nil, id
}

// hmailserver  创建邮箱账号
func (g *Imap) CreateAccount(username, password string) bool {

	return false
}
