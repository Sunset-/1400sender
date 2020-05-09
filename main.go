package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	jsoniter "github.com/json-iterator/go"
	"github.com/rcrowley/go-metrics"
	"github.com/spf13/viper"
	"io/ioutil"
	"net/http"
	"sunset/1400sender/concurrent"
	"time"
)


func InitConfig(configPath string) {
	fmt.Println("configPath:", configPath)
	viper.SetConfigFile(configPath)
	viper.SetConfigType("yaml")
	if err := viper.ReadInConfig(); err != nil {
		fmt.Println("Fail to read config file :", err)
	}
}

// 实际中应该用更好的变量名
var (
	h bool
	f string
	c string
	p int
)

func init() {

	flag.BoolVar(&h, "h", false, "this help")
	// 注意 `signal`。默认是 -s string，有了 `signal` 之后，变为 -s signal
	flag.StringVar(&f, "f", "", "config file path")
	flag.IntVar(&p, "p", 8769, "start port")

}

func main() {

	flag.Parse()

	fmt.Println("f:", f)

	if h {
		flag.Usage()
		return
	}
	//初始化配置
	if f != "" {
		InitConfig(f)
	}

	viewlib_id := viper.GetString("viewlib_id")

	header := map[string]string{
		"Content-Type" : "application/VIID+JSON",
		"User-Identify" : viewlib_id,
	}
	testData,err := ioutil.ReadFile(viper.GetString("data_file"))
	if err!=nil{
		fmt.Println(err)
		return
	}
	body := testData

	executor := concurrent.NewExecutor(10)

	//开启统计
	counter := metrics.NewMeter()
	counter.Mark(0)
	e := metrics.Register("send standard-collector data", counter)
	if e != nil {
		fmt.Println(e)
	}
	//日志打印
	go metrics.Log(metrics.DefaultRegistry, 5*time.Second, &MeterLog{})

	url := viper.GetString("http_request_url")
	for{
		executor.Submit(func(){
			err := request(url,http.MethodPost,"application/json",header,body)
			if err!=nil{
				fmt.Println(err)
			}else{
				//fmt.Println("发送成功")
				counter.Mark(int64(1))
			}
		})
	}
}


type MeterLog struct {
}

func (f *MeterLog) Printf(format string, v ...interface{}) {
	fmt.Println(fmt.Sprintf(format, v...))
}

var workerHttpClient = &http.Client{
	Transport: &http.Transport{
		MaxIdleConns:        20,
		MaxIdleConnsPerHost: 5,
		MaxConnsPerHost:     5,
		IdleConnTimeout:     30 * time.Second,
	},
	Timeout: 3 * time.Second,
}

//http请求
func request(url, method, contentType string,header map[string]string, body interface{}) error {
	var bodyBytes []byte
	var resBytes []byte
	if body != nil {
		if s,ok:=body.([]byte);ok{
			bodyBytes = []byte(s)
		}else{
			bodyBytes, _ = jsoniter.Marshal(body)
		}
	}
	//fmt.Println("http-request:", url)
	req, err := http.NewRequest(method, url, bytes.NewReader(bodyBytes))
	for k,v := range header{
		req.Header.Set(k,v)
	}
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", contentType)
	res, err := workerHttpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		err := res.Body.Close()
		if err != nil {
			fmt.Println("关闭res失败", err)
		}
	}()
	resBytes, err = ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return errors.New(string(resBytes))
	}
	return nil
}
