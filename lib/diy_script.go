package vegeta

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
)

//go:generate go run ../internal/cmd/jsonschema/script.go -type=Script -output=Script.schema.json
type Script struct {
	Workers         uint64    `json:"workers"`
	MaxWorkers      uint64    `json:"maxWorkers"`
	Duration        int       `json:"duration"`
	Debug           bool      `json:"debug,omitempty"`
	Rate            int       `json:"rate"`
	ParameterFile   string    `json:"parameterFile,omitempty"`
	ResultFile      string    `json:"resultFile,omitempty"`
	Requests        []Request `json:"requests"`
	Para            *Parameter
	ParaExist       bool
	ParameterHeader []string
}

const (
	DefaultName   = `diyAttack`
	DefaultOutput = `stdout`
)

type Request struct {
	Name      string     `json:"name"`
	Weight    int        `json:"weight"`
	DiyTarget *diyTarget `json:"target"`
	Target    *Target
}

type diyTarget struct {
	Method string      `json:"method"`
	URL    string      `json:"url"`
	Body   string      `json:"body,omitempty"`
	Header http.Header `json:"header,omitempty"`
}

const (
	DefaultRate     = 50
	DefaultDuration = 60
)

func newDefaultScript() Script {
	var sc Script
	sc.MaxWorkers = DefaultMaxWorkers
	sc.ResultFile = DefaultOutput
	sc.ParameterHeader = make([]string, 0)
	sc.Rate = DefaultRate
	sc.Duration = DefaultDuration
	sc.Debug = false

	return sc
}

func ParseScript(scriptFile string) *Script {
	sf, err := os.Open(scriptFile)
	if err != nil {
		fmt.Printf("the script file:%s not found.\n", scriptFile)
		os.Exit(1)
	}
	all, err := ioutil.ReadAll(sf)
	if err != nil {
		fmt.Printf("read the script file failed. err=%s\n", err)
		os.Exit(1)
	}

	script := newDefaultScript()
	err = json.Unmarshal(all, &script)
	if err != nil {
		fmt.Printf("unmarshal script file:%s failed. error=%s\n", scriptFile, err)
		os.Exit(1)
	}

	if len(script.Requests) < 1 {
		fmt.Println(ErrNoTargets)
		os.Exit(1)
	}

	for i := range script.Requests {

		if script.Requests[i].Name == "" {
			script.Requests[i].Name = DefaultName
		}

		if script.Requests[i].Weight == 0 {
			fmt.Println("request: weight must exits and must greater than 0")
			os.Exit(1)
		}

		if script.Requests[i].DiyTarget == nil {
			fmt.Println("request: target object is missing")
			os.Exit(1)
		}

		if script.Requests[i].DiyTarget.Method == "" {
			fmt.Println(ErrNoMethod)
			os.Exit(1)
		}

		if script.Requests[i].DiyTarget.URL == "" {
			fmt.Println(ErrNoURL)
			os.Exit(1)
		}

	}

	if script.ParameterFile != "" {
		p, err := NewParameter(true, script.ParameterFile)
		if err != nil {
			fmt.Printf("open parameters file failed. err=%s\n", err)
			os.Exit(1)
		}

		script.Para = p
		script.ParaExist = true
		line, err := p.Next()
		if err != nil {
			fmt.Printf("read the first line from %s error:%s\n", scriptFile, err)
			os.Exit(1)
		}
		headSli := strings.Split(line, ",")
		script.ParameterHeader = headSli
	} else {
		script.ParaExist = false
	}
	return &script
}

func (sc Script) UpdateParaData(m map[string]string) {
	if sc.ParaExist {
		data, _ := sc.Para.Next()
		dSli := strings.Split(data, ",")
		for j, v := range sc.ParameterHeader {
			m[v] = dSli[j]
		}
	}
}
