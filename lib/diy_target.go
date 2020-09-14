package vegeta

import (
	"bytes"
	"fmt"
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

			sc.Requests[i].DiyTarget.BodyBytes = []byte(sc.Requests[i].DiyTarget.Body)

		}

		wCon := NewWeightedControl(sc)

		return func(t *Target) (string, bool, error) {
			if t == nil {
				return DefaultName, false, ErrNilTarget
			}

			rand := wCon.Rand()
			t.Method = strings.ToUpper(sc.Requests[rand].DiyTarget.Method)
			if sc.Requests[rand].DiyTarget.Header != nil {
				t.Header = sc.Requests[rand].DiyTarget.Header
			}

			if sc.ParaExist {
				m := make(map[string]string)
				sc.UpdateParaData(m)
				data := bufferPool.Get().(*bytes.Buffer)
				urlTmpSli[rand].Execute(data, m)
				t.URL = data.String()
				data.Reset()

				bodyTmpSli[rand].Execute(data, m)
				bodyLen := data.Len()
				bodySli := make([]byte, bodyLen)
				copy(bodySli, data.Bytes())
				t.Body = bodySli

				data.Reset()
				bufferPool.Put(data)
			} else {
				t.URL = sc.Requests[rand].DiyTarget.URL
				t.Body = sc.Requests[rand].DiyTarget.BodyBytes
			}

			return sc.Requests[rand].Name, sc.Requests[rand].DisableKeepAlive, nil
		}
	}(script), script
}
