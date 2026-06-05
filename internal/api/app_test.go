package api

import (
	"reflect"
	"testing"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
)

// TestNewApp_WiresEveryField guards the DI container: every field of App must be
// populated by newApp. A forgotten handler (nil field) would panic at first
// request instead of failing here.
func TestNewApp_WiresEveryField(t *testing.T) {
	d, err := database.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	app := newApp(d, "/tmp/sshcm-test-config")
	if app == nil {
		t.Fatal("newApp returned nil")
	}

	v := reflect.ValueOf(*app)
	tp := v.Type()
	for i := 0; i < v.NumField(); i++ {
		f := v.Field(i)
		switch f.Kind() {
		case reflect.Ptr, reflect.Interface, reflect.Map, reflect.Func:
			if f.IsNil() {
				t.Errorf("App.%s is nil — newApp forgot to wire it", tp.Field(i).Name)
			}
		}
	}
}
