package client

import (
	"net/url"
	"net"
	"log"
	"fmt"
	"net/textproto"
	"bufio"
	"io"
	"bytes"
	"strconv"
	"strings"
	"github.com/lynMedia/libgortsp/comm"
)

/**
Rtsp Client
 */
type RtspClient struct {
	RtspUrl string//RTSP 地址
	UserName string
	Password string
	DebugModel bool
	rtspUrl *url.URL//
	requestUri string//格式化后地址
	tcpConn net.Conn//与服务端RTSP的TCP连接
	rconn io.Reader
	cseq uint //序号
	authorization string
	session string
	streams []AvStream
}
//请求命令
type requestCmd struct {
	Header []string
	Uri string
	Method string
}

type response struct {
	BlockLength int
	Block []byte
	BlockNo int

	StatusCode int
	Header textproto.MIMEHeader
	ContentLength int
	Body []byte
}

func (client *RtspClient) writeLine(line string) (err error) {
	if client.DebugModel {
		log.Println("C->S", line)
	}
	_, err = fmt.Fprint(client.tcpConn, line)
	return
}
func (client*RtspClient) writeRequest(req requestCmd)(err error)  {
	client.cseq++
	req.Header = append(req.Header, fmt.Sprintf("CSeq: %d", client.cseq))
	if err = client.writeLine(fmt.Sprintf("%s %s RTSP/1.0\r\n", req.Method, req.Uri)); err != nil {
		return
	}
	for _, v := range req.Header {
		if err = client.writeLine(fmt.Sprint(v, "\r\n")); err != nil {
			return
		}
	}
	if err = client.writeLine("\r\n"); err != nil {
		return
	}
	return
}

func (client *RtspClient) readResponse() (res response, err error) {
	var br *bufio.Reader

	defer func() {
		if br != nil {
			buf, _ := br.Peek(br.Buffered())
			client.rconn = io.MultiReader(bytes.NewReader(buf), client.rconn)
		}
		if res.StatusCode == 200 {
			if res.ContentLength > 0 {
				res.Body = make([]byte, res.ContentLength)
				if _, err = io.ReadFull(client.rconn, res.Body); err != nil {
					return
				}
			}
		} else if res.BlockLength > 0 {
			res.Block = make([]byte, res.BlockLength)
			if _, err = io.ReadFull(client.rconn, res.Block); err != nil {
				return
			}
		}
	}()

	var h [4]byte
	if _, err = io.ReadFull(client.rconn, h[:]); err != nil {
		return
	}

	if h[0] == 36 {
		// $
		res.BlockLength = int(h[2])<<8+int(h[3])
		res.BlockNo = int(h[1])
		if client.DebugModel {
			log.Println("C<-S block: len", res.BlockLength, "no", res.BlockNo)
		}
		return
	} else if h[0] == 82 && h[1] == 84 && h[2] == 83 && h[3] == 80 {
		// RTSP 200 OK
		client.rconn = io.MultiReader(bytes.NewReader(h[:]), client.rconn)
	} else {
		for {
			for {
				var b [1]byte
				if _, err = client.rconn.Read(b[:]); err != nil {
					return
				}
				if b[0] == 36 {
					break
				}
			}
			if client.DebugModel {
				log.Println("C<-S block: relocate")
			}
			if _, err = io.ReadFull(client.rconn, h[1:4]); err != nil {
				return
			}
			res.BlockLength = int(h[2])<<8+int(h[3])
			res.BlockNo = int(h[1])
			if res.BlockNo/2 < len(client.streams) {
				break
			}
		}
		if client.DebugModel {
			log.Println("C<-S block: len", res.BlockLength, "no", res.BlockNo)
		}
		return
	}

	br = bufio.NewReader(client.rconn)
	tp := textproto.NewReader(br)

	var line string
	if line, err = tp.ReadLine(); err != nil {
		return
	}
	if client.DebugModel {
		log.Println("C<-S", line)
	}

	fline := strings.SplitN(line, " ", 3)
	if len(fline) < 2 {
		err = fmt.Errorf("malformed RTSP response line")
		return
	}

	if res.StatusCode, err = strconv.Atoi(fline[1]); err != nil {
		return
	}
	var header textproto.MIMEHeader
	if header, err = tp.ReadMIMEHeader(); err != nil {
		return
	}

	if client.DebugModel {
		log.Println("C<-S", header)
	}

	if res.StatusCode != 200 && res.StatusCode != 401 {
		err = fmt.Errorf("rtsp: StatusCode=%d invalid", res.StatusCode)
		return
	}

	if res.StatusCode == 401 {
		/*
		RTSP/1.0 401 Unauthorized
		CSeq: 2
		Date: Wed, May 04 2016 10:10:51 GMT
		WWW-Authenticate: Digest realm="LIVE555 Streaming Media", nonce="c633aaf8b83127633cbe98fac1d20d87"
		*/
		authval := header.Get("WWW-Authenticate")
		hdrval := strings.SplitN(authval, " ", 2)
		var realm, nonce string

		if len(hdrval) == 2 {
			for _, field := range strings.Split(hdrval[1], ",") {
				field = strings.Trim(field, ", ")
				if keyval := strings.Split(field, "="); len(keyval) == 2 {
					key := keyval[0]
					val := strings.Trim(keyval[1], `"`)
					switch key {
					case "realm":
						realm = val
					case "nonce":
						nonce = val
					}
				}
			}

			if realm != "" && nonce != "" {
				if len(client.UserName)==0 {
					err = fmt.Errorf("rtsp: please provide username and password")
					return
				}
				var username string
				var password string
				username = client.UserName
				if len(client.Password)==0 {
					err = fmt.Errorf("rtsp: please provide password")
					return
				}
				password=client.Password
				hs1 := comm.Md5hash(username+":"+realm+":"+password)
				hs2 := comm.Md5hash("DESCRIBE:"+client.requestUri)
				response := comm.Md5hash(hs1+":"+nonce+":"+hs2)
				client.authorization = fmt.Sprintf(
					`Digest username="%s", realm="%s", nonce="%s", uri="%s", response="%s"`,
					username, realm, nonce, client.requestUri, response)
			}
		}
	}

	if sess := header.Get("Session"); sess != "" && client.session == "" {
		if fields := strings.Split(sess, ";"); len(fields) > 0 {
			client.session = fields[0]
		}
	}

	res.ContentLength, _ = strconv.Atoi(header.Get("Content-Length"))

	return
}

//连接RTSP服务
func (client*RtspClient) Connect() (err error) {
	var rtspUrl *url.URL
	if rtspUrl, err = url.Parse(client.RtspUrl); err != nil {
		log.Println("Rtsp Url Error", err)
		return
	}
	dailer := net.Dialer{}
	var conn net.Conn
	if conn, err = dailer.Dial("tcp", rtspUrl.Host); err != nil {
		log.Println("Connect", rtspUrl.Host, err)
		return
	}

	client.rtspUrl = rtspUrl;
	reqUri := *rtspUrl

	//提取用户名密码
	if len(reqUri.User.Username()) > 0 {
		client.UserName = reqUri.User.Username()
	}
	pass, set := reqUri.User.Password()
	if set && len(pass) > 0 {
		client.Password = pass
	}
	//提取请求地址
	reqUri.User = nil
	client.requestUri = reqUri.String()

	client.tcpConn = conn;
	client.rconn = conn;

	if client.DebugModel{
		log.Println("C->S Tcp Connect ",rtspUrl.Host," Succeed !")
	}

	return
}

func (client *RtspClient) Options() (err error) {
	if err = client.writeRequest(requestCmd{
		Method: "OPTIONS",
		Uri: client.requestUri,
	}); err != nil {
		return
	}
	if _, err = client.readResponse(); err != nil {
		return
	}
	return
}
