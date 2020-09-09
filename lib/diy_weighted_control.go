package vegeta

import (
	"math/rand"
	"sync"
)

var (
	one        sync.Once
	controller *Control
	lock       sync.Mutex
)

//Controller
type Control struct {
	current int
	data    []int
	total   int
	mul     sync.Mutex
}

func NewWeightedControl(sc *Script) *Control {

	if sc == nil {
		return nil
	}

	one.Do(func() {
		controller = &Control{
			current: 0,
			data:    make([]int, 0),
			total:   0,
		}

		for index, request := range sc.Requests {
			for i := 0; i < request.Weight; i++ {
				controller.data = append(controller.data, index)
				controller.total++
			}
		}

		controller.current = controller.total - 1
	})

	return controller
}

func (c *Control) Rand() int {
	lock.Lock()
	defer lock.Unlock()
	if c.current < 0 {
		c.current = c.total - 1
	}
	randIndex := rand.Intn(c.current + 1)
	result := c.data[randIndex]
	c.data[c.current], c.data[randIndex] = c.data[randIndex], c.data[c.current]
	c.current--
	return result
}
