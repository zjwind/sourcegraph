package worker

import (
	"context"
	"sort"
	"sync"
)

type IDSet struct {
	sync.RWMutex
	ids map[int]context.CancelFunc
}

func newIDSet() *IDSet {
	return &IDSet{ids: map[int]context.CancelFunc{}}
}

func (i *IDSet) Add(id int, cancel context.CancelFunc) bool {
	i.Lock()
	defer i.Unlock()

	if _, ok := i.ids[id]; ok {
		return false
	}

	i.ids[id] = cancel
	return true
}

func (i *IDSet) Remove(id int) {
	i.Lock()
	cancel, ok := i.ids[id]
	delete(i.ids, id)
	i.Unlock()

	if ok {
		cancel()
	}
}

func (i *IDSet) Slice() []int {
	i.RLock()
	defer i.RUnlock()

	ids := make([]int, 0, len(i.ids))
	for id := range i.ids {
		ids = append(ids, id)
	}
	sort.Ints(ids)

	return ids
}
