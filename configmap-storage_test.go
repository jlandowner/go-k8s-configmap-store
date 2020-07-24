package storage

import (
	"fmt"
	"reflect"
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
			name: "Increase localMap",
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
		{
			name: "Decrease localMap",
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
					Data:       map[string]string{"testdata1": "foo"},
				},
			},
		},
	}

	for _, test := range tests {
		l := new(sync.Mutex)
		wg := new(sync.WaitGroup)
		c := sync.NewCond(l)

		for _, l := range test.localMap {
			t.Log("Before:", l.configMap.Name, l.configMap.Data)
		}

		syncLocalMap(test.localMap, test.syncList)

		for _, mapStorage := range test.localMap {
			wg.Add(1)
			t.Log("After:", mapStorage.configMap.Name)
			localcm := mapStorage.configMap
			go func(localcm *corev1.ConfigMap, syncList []*corev1.ConfigMap) {
				fmt.Println(localcm.Name, localcm.Data)
				l.Lock()
				defer l.Unlock()
				c.Wait()
				fmt.Println("GO!")

				ok := false
				for _, sync := range syncList {
					if reflect.DeepEqual(*localcm, *sync) {
						ok = true
					}
				}
				if !ok {
					t.Errorf("Not match in %s", localcm.Name)
				}
				wg.Done()
			}(localcm, test.syncList)
		}
		time.Sleep(3 * time.Second)
		c.Broadcast()
		wg.Wait()
	}
}
