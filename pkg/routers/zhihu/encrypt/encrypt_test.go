package encrypt

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"

	"github.com/dop251/goja"
	"github.com/robertkrimen/otto"
	v8 "rogchap.com/v8go"
)

func TestV8Go(t *testing.T) {
	t.Log("TestV8Go")

	jsCode, err := os.ReadFile("encrypt.js")
	if err != nil {
		t.Fatal(err)
	}

	ctx := v8.NewContext()

	_, err = ctx.RunScript(string(jsCode), "script.js")
	if err != nil {
		t.Fatal(err)
	}

	md5Value := "123456"
	result, err := ctx.RunScript(fmt.Sprintf("getEncryptCode(%s)", md5Value), "main.js")
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(result)
}

func TestGoja(t *testing.T) {
	t.Log("TestGoja")

	jsCode, err := os.ReadFile("encrypt.js")
	if err != nil {
		t.Fatal(err)
	}

	vm := goja.New()

	_, err = vm.RunString(string(jsCode))
	if err != nil {
		t.Fatal(err)
	}

	var md5Value string = "1234324"
	result, err := vm.RunString(fmt.Sprintf("getEncryptCode('%s')", md5Value))
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println("Result:", result)
}

func TestOTTO(t *testing.T) {
	t.Log("TestOTTO")

	jsCode, err := os.ReadFile("encrypt.js")
	if err != nil {
		t.Fatal(err)
	}

	vm := otto.New()

	_, err = vm.Run(string(jsCode))
	if err != nil {
		t.Fatal(err)
	}

	var md5Value string = "your_md5_value_here"
	value, err := vm.Call("getEncryptCode", nil, md5Value)
	if err != nil {
		t.Fatal(err)
	}

	if result, err := value.ToString(); err == nil {
		fmt.Println("Result:", result)
	} else {
		t.Fatal(err)
	}
}

func TestNodeServer(t *testing.T) {
	t.Log("TestNodeServer")

	resp, err := http.Get("http://localhost:3000/?md5=123434")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(string(body))
}

const (
	baseURL    = "https://www.zhihu.com"
	nodeServer = "http://localhost:3000"
	x_zse_93   = "101_3_3.0"
)

func getDC0() (string, error) {
	req, err := http.NewRequest("POST", baseURL+"/udid", nil)
	if err != nil {
		return "", err
	}
	req.Header.Add("x-zse-93", x_zse_93)
	req.Header.Add("x-api-version", "3.0.91")
	req.Header.Add("user-agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/107.0.0.0 Safari/537.36")
	a_v, err := getEncryptCode(x_zse_93+"/udid", "")
	if err != nil {
		return "", err
	}
	req.Header.Add("x-zse-96", "2.0_"+a_v)
	req.Header.Add("accept", "*/*")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	fmt.Printf("getDC0 body: %s\n", string(body))

	cookies := resp.Cookies()
	fmt.Printf("cookies: %+v", cookies)
	for _, cookie := range cookies {
		if cookie.Name == "d_c0" {
			return cookie.Value, nil
		}
	}
	return "", fmt.Errorf("d_c0 not found")
}

func getEncryptCode(urlPath string, dC0 string) (string, error) {
	baseStr := x_zse_93 + "+" + urlPath + "+" + dC0
	fmt.Printf("getEncryptCode baseStr: %s\n", baseStr)
	hasher := md5.New()
	hasher.Write([]byte(baseStr))
	md5Str := hex.EncodeToString(hasher.Sum(nil))

	encryptURL := fmt.Sprintf("%s/?md5=%s", nodeServer, md5Str)
	resp, err := http.Get(encryptURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	fmt.Println(string(body))

	return string(body), nil
}

func TestZhiHuEncrypt(t *testing.T) {
	dC0, err := getDC0()
	if err != nil {
		t.Fatalf("Error getting d_c0: %v", err)
	}

	path := "/api/v4/members/lai-si-shuang-yu-27/followers?"
	param := "include=data%5B*%5D.answer_count%2Carticles_count%2Cgender%2Cfollower_count%2Cis_followed%2Cis_following%2Cbadge%5B%3F%28type%3Dbest_answerer%29%5D.topics&offset=220&limit=20"
	encryptStr, err := getEncryptCode(path+param, dC0)
	if err != nil {
		t.Fatalf("Error getting encrypted string: %v", err)
	}
	fmt.Println("encryptStr for final req:", encryptStr)

	finalURL := baseURL + path + param
	req, _ := http.NewRequest("GET", finalURL, nil)
	req.Header.Add("x-zse-96", "2.0_"+encryptStr)
	req.Header.Add("cookie", "d_c0="+dC0+";")
	req.Header.Add("accept-encoding", "gzip, deflate, br")
	req.Header.Add("accept-language", "zh-CN,zh;q=0.9")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Error sending request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Error reading response body: %v", err)
	}

	fmt.Println(string(body))
}
