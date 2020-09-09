package vegeta

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"text/template"
)

type DiyError struct {
	Name string
	text string
}

func (d DiyError) Error() string {
	return d.text
}

func NewWeightTargeter(scriptFile string) (DiyTargeter, *Script) {

	script := ParseScript(scriptFile)

	return func(sc *Script) DiyTargeter {

		nums := len(sc.Requests)

		urlTmpSli := make([]*template.Template, nums)
		bodyTmpSli := make([]*template.Template, nums)
		bufferPool := sync.Pool{
			New: func() interface{} {
				return &bytes.Buffer{}
			},
		}
		for i := 0; i < nums; i++ {
			urlTmp := template.Must(template.New(fmt.Sprintf("url_%d", i)).Parse(sc.Requests[i].DiyTarget.URL))
			urlTmpSli[i] = urlTmp

			bodyTmp := template.Must(template.New(fmt.Sprintf("url_%d", i)).Parse(sc.Requests[i].DiyTarget.Body))
			bodyTmpSli[i] = bodyTmp

		}

		wCon := NewWeightedControl(sc)

		return func(t *Target) (string, error) {
			if t == nil {
				return DefaultName, ErrNilTarget
			}
			m := make(map[string]string)
			rand := wCon.Rand()
			sc.UpdateParaData(m)

			data := bufferPool.Get().(*bytes.Buffer)
			urlTmpSli[rand].Execute(data, m)
			t.Method = strings.ToUpper(sc.Requests[rand].DiyTarget.Method)
			t.URL = data.String()
			t.Header = http.Header{}
			if sc.Requests[rand].DiyTarget.Header != nil {
				t.Header = sc.Requests[rand].DiyTarget.Header
			}

			data.Reset()

			bodyTmpSli[rand].Execute(data, m)
			t.Body = data.Bytes()

			data.Reset()
			bufferPool.Put(data)

			return sc.Requests[rand].Name, nil
		}
	}(script), script
}
