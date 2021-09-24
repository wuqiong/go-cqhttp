package server

import (
	"bytes"
	"fmt"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Mrs4s/go-cqhttp/coolq"
	"github.com/Mrs4s/go-cqhttp/global"
	"github.com/Mrs4s/go-cqhttp/global/config"

	"github.com/Mrs4s/MiraiGo/utils"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
)



// FortuneAgent WebSocket客户端实例
type fortuneClient struct {
	bot  *coolq.CQBot
	conf *config.WebsocketFortune

	universalConn *fortuneAgentConn
	token         string
	filter        string
}

type fortuneAgentConn struct {
	*websocket.Conn
	sync.Mutex
	apiCaller *apiCaller
}


// RunFortuneClient 运行一个正向WS client
func RunFortuneClient(b *coolq.CQBot, conf *config.WebsocketFortune) {
	if conf.Disabled {
		return
	}
	c := &fortuneClient{
		bot:    b,
		conf:   conf,
		token:  conf.AccessToken,
		filter: conf.Filter,
	}
	addFilter(c.filter)
	if c.conf.Url != "" {
		c.connectUniversal()
	}
	c.bot.OnEventPush(c.onBotPushEvent)
}

func (c *fortuneClient) connectUniversal() {
	log.Infof("开始尝试连接到FortuneAgent工具WS服务器: %v", c.conf.Url)
	header := http.Header{
		"X-Client-Role": []string{"FortuneAgent-QQClient"},
		"X-Self-ID":     []string{strconv.FormatInt(c.bot.Client.Uin, 10)},
		"User-Agent":    []string{"CQHttp/4.15.0"},
	}
	if c.token != "" {
		header["Authorization"] = []string{"Token " + c.token}
	}
connectWS:
	conn, _, err := websocket.DefaultDialer.Dial(c.conf.Url, header) // nolint
	if err != nil {
		log.Warnf("连接到FortuneAgent工具WebSocket服务器 %v 时出现错误: %v", c.conf.Url, err)
		if c.conf.ReconnectInterval != 0 {
			time.Sleep(time.Millisecond * time.Duration(c.conf.ReconnectInterval))
			//c.connectUniversal()
			goto connectWS
		}
		return
	}
	handshake := fmt.Sprintf(`{"type":"event","payload":{"meta_event_type":"lifecycle","post_type":"meta_event","self_id":%d,"nickname":"%s","sub_type":"connect","time":%d}}`,
		c.bot.Client.Uin, c.bot.Client.Nickname, time.Now().Unix())
	err = conn.WriteMessage(websocket.TextMessage, []byte(handshake))
	if err != nil {
		log.Warnf("连接到FortuneAgent工具WebSocket 握手时出现错误: %v", err)
	}

	if c.universalConn == nil {
		wrappedConn := &fortuneAgentConn{Conn: conn, apiCaller: newAPICaller(c.bot)}
		if c.conf.RateLimit.Enabled {
			wrappedConn.apiCaller.use(rateLimit(c.conf.RateLimit.Frequency, c.conf.RateLimit.Bucket))
		}
		c.universalConn = wrappedConn
	} else {
		c.universalConn.Conn = conn
	}
	go c.listenAPI(c.universalConn, false)
}

func (c *fortuneClient) listenAPI(conn *fortuneAgentConn, u bool) {
	defer func() { _ = conn.Close() }()
	for {
		buffer := global.NewBuffer()
		t, reader, err := conn.NextReader()
		if err != nil {
			log.Warnf("监听FortuneAgent工具WS API时出现错误1: %v", err)
			break
		}
		_, err = buffer.ReadFrom(reader)
		if err != nil {
			log.Warnf("监听FortuneAgent工具WS API时出现错误2: %v", err)
			break
		}
		if t == websocket.TextMessage {
			go func(buffer *bytes.Buffer) {
				defer global.PutBuffer(buffer)
				conn.handleRequest(c.bot, buffer.Bytes())
			}(buffer)
		} else {
			global.PutBuffer(buffer)
		}
	}
	if c.conf.ReconnectInterval != 0 {
		time.Sleep(time.Millisecond * time.Duration(c.conf.ReconnectInterval))
		if !u {
			//go c.connectAPI()
			go c.connectUniversal()
		}
	}
}

func (c *fortuneClient) onBotPushEvent(e *coolq.Event) {
	filter := findFilter(c.filter)
	if filter != nil && !filter.Eval(gjson.Parse(e.JSONString())) {
		log.Debugf("上报Event %s 到 FortuneAgent工具WS服务器 时被过滤.", e.JSONBytes())
		return
	}
	push := func(conn *fortuneAgentConn, reconnect func()) {
		log.Debugf("向FortuneAgent工具WS服务器 %v 推送Event: %s", conn.RemoteAddr().String(), e.JSONBytes())
		conn.Lock()
		defer conn.Unlock()
		_ = conn.SetWriteDeadline(time.Now().Add(time.Second * 15))
		eventMsg := coolq.MSG{
			"type":"event",
			"payload": e.RawMsg,
		}
		if err := conn.WriteJSON(eventMsg); err != nil {
			log.Warnf("向FortuneAgent工具WS服务器 %v 推送Event时出现错误: %v", conn.RemoteAddr().String(), err)
			_ = conn.Close()
			if c.conf.ReconnectInterval != 0 {
				time.Sleep(time.Millisecond * time.Duration(c.conf.ReconnectInterval))
				reconnect()
			}
		}
	}
	if c.universalConn != nil {
		push(c.universalConn, c.connectUniversal)
	}
}


func (c *fortuneAgentConn) handleRequest(_ *coolq.CQBot, payload []byte) {
	defer func() {
		if err := recover(); err != nil {
			log.Printf("处置FortuneAgent工具WS命令时发生无法恢复的异常：%v\n%s", err, debug.Stack())
			_ = c.Close()
		}
	}()
	j := gjson.Parse(utils.B2S(payload))
	t := strings.TrimSuffix(j.Get("action").Str, "_async")
	log.Debugf("WS接收到FortuneAgent工具WS API调用: %v 参数: %v", t, j.Get("params").Raw)
	ret := c.apiCaller.callAPI(t, j.Get("params"))
	if j.Get("echo").Exists() {
		ret["echo"] = j.Get("echo").Value()
	}
	respMsg := coolq.MSG {
		"type":"api",
		"payload":ret,
	}
	if j.Get("callback").Exists() {
		respMsg["callback"] = j.Get("callback").Value()
	}else{
		respMsg["callback"] = ""
	}
	c.Lock()
	defer c.Unlock()
	_ = c.WriteJSON(respMsg)
}

