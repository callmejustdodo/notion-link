package resolve

import "testing"

func TestPageID(t *testing.T) {
	const want = "2bd2cfba-780a-806c-b320-e65c4f924ae7"
	cases := []struct {
		name, in, want string
		wantErr        bool
	}{
		{"dashed", "2bd2cfba-780a-806c-b320-e65c4f924ae7", want, false},
		{"compact", "2bd2cfba780a806cb320e65c4f924ae7", want, false},
		{"upper-dashed", "2BD2CFBA-780A-806C-B320-E65C4F924AE7", want, false},
		{"notion-url", "https://www.notion.so/Some-Title-2bd2cfba780a806cb320e65c4f924ae7", want, false},
		{"notion-url-slash", "https://www.notion.so/Some-Title-2bd2cfba780a806cb320e65c4f924ae7/", want, false},
		{"notion-app-scheme", "notion://www.notion.so/2bd2cfba780a806cb320e65c4f924ae7", want, false},
		{"empty", "", "", true},
		{"garbage", "not-a-page-id", "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := PageID(c.in)
			if (err != nil) != c.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, c.wantErr)
			}
			if got != c.want {
				t.Fatalf("got %q want %q", got, c.want)
			}
		})
	}
}
