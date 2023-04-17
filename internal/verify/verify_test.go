package verify

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"testing"
)

func Test_verifyWithKeys(t *testing.T) {
	var pk []string
	{
		file, err := os.Open("test_data/public.key")
		if err != nil {
			panic(err)
		}
		defer file.Close()

		// Get last line (key string) from file
		scanner := bufio.NewScanner(file)
		for i := 0; i < 2; i++ {
			if !scanner.Scan() {
				panic(scanner.Err())
			}
		}
		pk = []string{scanner.Text()}
	}

	tests := []struct {
		expectedErrPrefix string
		testName          string
		signatureFile     string
		jsonFile          string
		expectedFileName  string
		minSignTime       uint64
		allowedPks        []string
	}{
		{
			"invalid signature algorithm '",
			"pure",
			"server_list.json.pure.minisig",
			"server_list.json",
			"server_list.json",
			10,
			pk,
		},
		{
			"",
			"valid server_list",
			"server_list.json.minisig",
			"server_list.json",
			"server_list.json",
			10,
			pk,
		},
		{
			"",
			"TC no hashed",
			"server_list.json.tc_nohashed.minisig",
			"server_list.json",
			"server_list.json",
			10,
			pk,
		},
		{
			"",
			"TC later time",
			"server_list.json.tc_latertime.minisig",
			"server_list.json",
			"server_list.json",
			10,
			pk,
		},
		{
			"wrong filename '",
			"server_list TC file:organization_list",
			"server_list.json.tc_orglist.minisig",
			"server_list.json",
			"server_list.json",
			10,
			pk,
		},
		{
			"wrong filename '",
			"organization_list as server_list",
			"organization_list.json.minisig",
			"organization_list.json",
			"server_list.json",
			10,
			pk,
		},
		{
			"wrong filename '",
			"TC file:otherfile",
			"server_list.json.tc_otherfile.minisig",
			"server_list.json",
			"server_list.json",
			10,
			pk,
		},
		{
			"invalid trusted comment '",
			"TC no file",
			"server_list.json.tc_nofile.minisig",
			"server_list.json",
			"server_list.json",
			10,
			pk,
		},
		{
			"invalid trusted comment '",
			"TC no time",
			"server_list.json.tc_notime.minisig",
			"server_list.json",
			"server_list.json",
			10,
			pk,
		},
		{
			"invalid trusted comment '",
			"TC empty time",
			"server_list.json.tc_emptytime.minisig",
			"server_list.json",
			"server_list.json",
			10,
			pk,
		},
		{
			"wrong filename '",
			"TC empty file",
			"server_list.json.tc_emptyfile.minisig",
			"server_list.json",
			"server_list.json",
			10,
			pk,
		},
		{
			"invalid trusted comment '",
			"TC random",
			"server_list.json.tc_random.minisig",
			"server_list.json",
			"server_list.json",
			10,
			pk,
		},
		{
			"",
			"large time",
			"server_list.json.large_time.minisig",
			"server_list.json",
			"server_list.json",
			43e8,
			pk,
		},
		{
			"",
			"lower min time",
			"server_list.json.minisig",
			"server_list.json",
			"server_list.json",
			5,
			pk,
		},
		{
			"sign time",
			"higher min time",
			"server_list.json.minisig",
			"server_list.json",
			"server_list.json",
			11,
			pk,
		},
		{
			"",
			"valid organization_list",
			"organization_list.json.minisig",
			"organization_list.json",
			"organization_list.json",
			10,
			pk,
		},
		{
			"wrong filename '",
			"organization_list TC file:server_list",
			"organization_list.json.tc_servlist.minisig",
			"organization_list.json",
			"organization_list.json",
			10,
			pk,
		},
		{
			"wrong filename '",
			"server_list as organization_list",
			"server_list.json.minisig",
			"server_list.json",
			"organization_list.json",
			10,
			pk,
		},

		{
			"invalid filename '",
			"valid other_list",
			"other_list.json.minisig",
			"other_list.json",
			"other_list.json",
			10,
			pk,
		},
		{
			"wrong filename '",
			"other_list as server_list",
			"other_list.json.minisig",
			"other_list.json",
			"server_list.json",
			10,
			pk,
		},
		{
			"invalid signature format",
			"invalid signature file",
			"random.txt",
			"server_list.json",
			"server_list.json",
			10,
			pk,
		},
		{
			"invalid signature format",
			"empty signature file",
			"empty",
			"server_list.json",
			"server_list.json",
			10,
			pk,
		},

		{
			"signature for filename '",
			"wrong key",
			"server_list.json.wrong_key.minisig",
			"server_list.json",
			"server_list.json",
			10,
			pk,
		},

		{
			"invalid signature algorithm '",
			"forged pure signature",
			"server_list.json.forged_pure.minisig",
			"server_list.json.blake2b",
			"server_list.json",
			10,
			pk,
		},
		{
			"invalid signature",
			"forged key ID",
			"server_list.json.forged_keyid.minisig",
			"server_list.json",
			"server_list.json",
			10,
			pk,
		},

		{
			"signature for filename '",
			"no allowed keys",
			"server_list.json.minisig",
			"server_list.json",
			"server_list.json",
			10,
			[]string{},
		},
		{
			"",
			"multiple allowed keys 1",
			"server_list.json.minisig",
			"server_list.json",
			"server_list.json",
			10,
			[]string{
				pk[0], "RWSf0PYToIUJmDlsz21YOXvgQzHj9NSdyJUqEY5ZdfS9GepeXt3+JJRZ",
			},
		},
		{
			"",
			"multiple allowed keys 2",
			"server_list.json.minisig",
			"server_list.json",
			"server_list.json",
			10,
			[]string{
				"RWSf0PYToIUJmDlsz21YOXvgQzHj9NSdyJUqEY5ZdfS9GepeXt3+JJRZ", pk[0],
			},
		},
		{
			"failed to create public key '",
			"invalid allowed key",
			"server_list.json.minisig",
			"server_list.json",
			"server_list.json",
			10,
			[]string{"AAA"},
		},
	}

	// Cache file contents in map, mapping file names to contents
	files := map[string][]byte{}
	loadFile := func(name string) {
		_, loaded := files[name]
		if !loaded {
			content, err := os.ReadFile("test_data/" + name)
			if err != nil {
				panic(err)
			}
			files[name] = content
		}
	}
	for _, test := range tests {
		loadFile(test.signatureFile)
		loadFile(test.jsonFile)
	}

	forcePrehash := true
	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			valid, err := verifyWithKeys(string(files[tt.signatureFile]), files[tt.jsonFile],
				tt.expectedFileName, tt.minSignTime, tt.allowedPks, forcePrehash)
			compareResults(t, valid, err, tt.expectedErrPrefix, func() string {
				return fmt.Sprintf(
					"verifyWithKeys(%q, %q, %q, %v, %v, %t)",
					tt.signatureFile,
					tt.jsonFile,
					tt.expectedFileName,
					tt.minSignTime,
					tt.allowedPks,
					forcePrehash,
				)
			})
		})
	}
}

// compareResults compares returned ret, err from a verify function with expected error code expected.
// callStr is called to get the formatted parameters passed to the function.
func compareResults(
	t *testing.T,
	ret bool,
	err error,
	expectedErrPrefix string,
	callStr func() string,
) {
	if expectedErrPrefix == "" {
		// we don't expect any error
		if err != nil {
			t.Errorf("error not expected but returned '%s', callstr '%s'", err.Error(), callStr())
		}
		if !ret {
			t.Errorf("error is nil and result is false, callstr: '%s'", callStr())
		}
		return
	}

	if err == nil {
		// we expect an error but received nil
		t.Errorf("expected error prefix '%s' but received nil, callstr: '%s'", expectedErrPrefix, callStr())
		return
	}

	if !strings.HasPrefix(err.Error(), expectedErrPrefix) {
		// wrong error
		t.Errorf("expected error prefix '%s' for error '%s', callstr: '%s'", expectedErrPrefix, err.Error(), callStr())
		return
	}

	if ret {
		t.Errorf("error is not nil and result is true, callstr: '%s'", callStr())
	}
}
