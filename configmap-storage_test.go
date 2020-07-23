package storage

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestExtractBaseName(t *testing.T) {
	tests := []struct {
		name   string
		expect string
	}{
		{
			name:   namePrefix + "." + "aaa",
			expect: "aaa",
		},
	}

	for _, test := range tests {
		t.Log(test.name)
		assert.Equal(t, test.expect, extractBaseName(test.name))
	}
}

func TestSyncLocalMap(t *testing.T) {
	tests := []struct {
		name     string
		localMap map[string]*MapStorage
		syncList []*corev1.ConfigMap
	}{
		{
			name: "test1",
			localMap: map[string]*MapStorage{
				"foo": {configMap: &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Name: namePrefix + "." + "foo"},
					Data:       map[string]string{"testdata1": "foo"},
				}, lock: new(sync.RWMutex)},
				"bar": {configMap: &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Name: namePrefix + "." + "bar"},
					Data:       map[string]string{"testdata1": "bar"},
				}, lock: new(sync.RWMutex)},
			},
			syncList: []*corev1.ConfigMap{
				{
					ObjectMeta: metav1.ObjectMeta{Name: namePrefix + "." + "foo"},
					Data:       map[string]string{"testdata1": "bar"},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: namePrefix + "." + "bar"},
					Data:       map[string]string{"testdata1": "bar", "testdata2": "bar2"},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: namePrefix + "." + "foobar"},
					Data:       map[string]string{"testdata1": "foobar"},
				},
			},
		},
	}

	for _, test := range tests {
		l := new(sync.Mutex)
		wg := new(sync.WaitGroup)
		c := sync.NewCond(l)

		syncLocalMap(test.localMap, test.syncList)

		for _, cm := range test.syncList {
			wg.Add(1)
			t.Log(cm.Name)
			a := cm
			go func(cm *corev1.ConfigMap) {
				fmt.Println(cm.Name, cm.Data)
				l.Lock()
				defer l.Unlock()
				c.Wait()
				fmt.Println("GO!")

				data, exist := test.localMap[extractBaseName(cm.Name)]
				assert.True(t, exist)
				assert.Equal(t, *cm, *data.configMap)
				wg.Done()
			}(a)
		}
		time.Sleep(3 * time.Second)
		c.Broadcast()
		wg.Wait()
	}
}
