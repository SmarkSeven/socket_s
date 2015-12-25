package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"time"
)

func reader(readerChannel chan []byte) {
	for {
		select {
		case data := <-readerChannel:
			Log(string(data))
		}
	}
}

func CheckError(err error) {
	if err != nil {
		log.Printf("Fatal error: %s", err.Error())
		os.Exit(1)
	}
}

type Response struct {
	StatusCode string      `json:"statusCode"`
	Result     interface{} `result`
}

//长连接
func handleConnection(conn net.Conn, timeout int) {
	buffer := make([]byte, 1024)
	for {
		n, err := conn.Read(buffer)
		if err != nil {
			Log(conn.RemoteAddr().String(), " connection error: ", err)
			return
		}
		data := (buffer[:n])
		messnager := make(chan byte, 100)
		// //心跳计时
		go HeartBeating(conn, messnager, timeout)
		//检测每次Client是否有数据传来
		go GravelChannel(data, messnager)
		flag := false
		for _, v := range routers {
			pred := v[0]
			act := v[1]
			var message Msg
			err := json.Unmarshal(data, &message)
			if err != nil {
				Log(err)
			}
			if pred.(func(entry Msg) bool)(message) {
				result := act.(Controller).Excute(message)
				_, err := WriteResult(conn, result)
				if err != nil {
					Log("conn.WriteResult()", err)
				}
				flag = true
				break
			}
		}
		if !flag {
			_, err := WriteError(conn, "1111", "不能处理此类型的业务")
			if err != nil {
				Log("conn.WriteError()", err)
			}
		}
	}
}

func WriteResult(conn net.Conn, result interface{}) (n int, err error) {
	data, err := json.Marshal(Response{StatusCode: "0000", Result: result})
	if err != nil {
		return 0, err
	}
	return conn.Write(data)
}

func WriteError(conn net.Conn, statusCode string, result interface{}) (n int, err error) {
	data, err := json.Marshal(Response{StatusCode: statusCode, Result: result})
	if err != nil {
		return 0, err
	}
	return conn.Write(data)
}

//心跳计时，根据GravelChannel判断Client是否在设定时间内发来信息
func HeartBeating(conn net.Conn, readerChannel chan byte, timeout int) {
	select {
	case <-readerChannel:
		conn.SetDeadline(time.Now().Add(time.Duration(timeout) * time.Second))
		break
	case <-time.After(time.Second * time.Duration(timeout)):
		Log("It's really weird to get Nothing!!!")
		conn.Close()
	}

}

func GravelChannel(n []byte, mess chan byte) {
	for _, v := range n {
		mess <- v
	}
	close(mess)
}

func Log(v ...interface{}) {
	log.Println(v...)
}

type Msg struct {
	Conditions map[string]string `json:"meta"`
	Content    interface{}       `json:"content"`
}

type Controller interface {
	Excute(message Msg) interface{}
}

var routers [][2]interface{}

func Route(judge interface{}, controller Controller) {
	switch judge.(type) {
	case func(entry Msg) bool:
		{
			var arr [2]interface{}
			arr[0] = judge
			arr[1] = controller
			routers = append(routers, arr)
		}
	case map[string]string:
		{
			defaultJudge := func(entry Msg) bool {
				for keyjudge, valjudge := range judge.(map[string]string) {
					val, ok := entry.Conditions[keyjudge]
					if !ok {
						return false
					}
					if val != valjudge {
						return false
					}
				}
				return true
			}
			var arr [2]interface{}
			arr[0] = defaultJudge
			arr[1] = controller
			routers = append(routers, arr)
			fmt.Println(routers)
		}
	default:
		log.Println("Something is wrong in Router")
	}
}

type MirrorController struct {
}

func (this *MirrorController) Excute(message Msg) interface{} {
	_, err := json.Marshal(message)
	CheckError(err)
	return "消息推送成功"
}
func mirrorHandle(entry Msg) bool {
	if entry.Conditions["msgtype"] == "binding" {
		return true
	}
	return false
}

func init() {
	var mirror MirrorController
	routers = make([][2]interface{}, 0, 10)
	kvs := make(map[string]string)
	kvs["msgtype"] = "binding"
	Route(kvs, &mirror)
}

func main() {
	netListen, err := net.Listen("tcp", "localhost:6060")
	CheckError(err)
	defer netListen.Close()
	Log("Waiting for clients")
	for {
		conn, err := netListen.Accept()
		if err != nil {
			continue
		}
		Log(conn.RemoteAddr().String(), " tcp connect success")
		go handleConnection(conn, 2)
	}
}
