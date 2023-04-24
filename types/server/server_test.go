package server

import (
	"encoding/json"
	"testing"
)

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func TestServerUnmarshal(t *testing.T) {
	cases := []struct{
		want Type
		wantErr string
		payload string
	}{
		{
			want: TypeUnknown,
			wantErr: "invalid server type: a",
			payload: `{"value": "a"}`,
		},
		{
			want: TypeInstituteAccess,
			wantErr: "",
			payload: `{"value": "institute_access"}`,
		},
		{
			want: TypeCustom,
			wantErr: "",
			payload: `{"value": "custom_server"}`,
		},
		{
			want: TypeSecureInternet,
			wantErr: "",
			payload: `{"value": "secure_internet"}`,
		},
		{
			want: TypeUnknown,
			wantErr: "",
			payload: `{"value": 0}`,
		},
		{
			want: TypeInstituteAccess,
			wantErr: "",
			payload: `{"value": 1}`,
		},
		{
			want: TypeSecureInternet,
			wantErr: "",
			payload: `{"value": 2}`,
		},
		{
			want: TypeCustom,
			wantErr: "",
			payload: `{"value": 3}`,
		},
		// Values that are outside the range will be error checked too
		// This is thus even more strict than a regular type unmarshal/marshal
		{
			want: TypeUnknown,
			wantErr: "invalid server type: 25",
			payload: `{"value": 25}`,
		},
	}

	for _, c := range cases {
		var got struct{
			Value Type `json:"value"`
		}
		err := json.Unmarshal([]byte(c.payload), &got)
		if errorString(err) != c.wantErr {
			t.Fatalf("server unmarshal error is not equal to want, got: %v, want: %v", err, c.want)
		}
		if got.Value != c.want {
			t.Fatalf("server unmarshal value is not equal to want, got: %v, want: %v", got.Value, c.want)
		}
	}
		
}
