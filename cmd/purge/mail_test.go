package main

import (
	"html/template"
	"os"
	"testing"
	"time"

	"github.com/cloudfoundry-community/go-cfclient/v3/resource"
	"github.com/google/go-cmp/cmp"
)

func TestRenderTemplate(t *testing.T) {
	notifyTemplate, err := template.ParseFiles("../../templates/base.html", "../../templates/notify.tmpl")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	purgeTemplate, err := template.ParseFiles("../../templates/base.html", "../../templates/purge.tmpl")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	testCases := map[string]struct {
		tpl              *template.Template
		data             map[string]interface{}
		expectedErr      string
		expectedTestFile string
	}{
		"constructs the appropriate notify template": {
			tpl: notifyTemplate,
			data: map[string]interface{}{
				"org": &resource.Organization{
					Name: "test-org",
				},
				"space": &resource.Space{
					Name: "test-space",
				},
				"date": time.Date(2009, 11, 17, 20, 34, 58, 651387237, time.UTC),
				"days": 90,
			},
			expectedTestFile: "../../testdata/notify.html",
		},
		"constructs the appropriate purge template": {
			tpl: purgeTemplate,
			data: map[string]interface{}{
				"org": &resource.Organization{
					Name: "test-org",
				},
				"space": &resource.Space{
					Name: "test-space",
				},
				"date": time.Date(2009, 11, 17, 20, 34, 58, 651387237, time.UTC),
				"days": 90,
			},
			expectedTestFile: "../../testdata/purge.html",
		},
	}
	for name, test := range testCases {
		t.Run(name, func(t *testing.T) {
			renderedTemplate, err := renderTemplate(
				test.tpl,
				test.data,
			)
			if (test.expectedErr == "" && err != nil) || (test.expectedErr != "" && test.expectedErr != err.Error()) {
				t.Fatalf("expected error: %s, got: %s", test.expectedErr, err)
			}
			if test.expectedTestFile != "" {
				if os.Getenv("OVERRIDE_TEMPLATES") == "1" {
					err := os.WriteFile(test.expectedTestFile, []byte(renderedTemplate), 0644)
					if err != nil {
						t.Fatalf("unexpected error: %s", err)
					}
				}
				expected, err := os.ReadFile(test.expectedTestFile)
				if err != nil {
					t.Fatalf("unexpected error: %s", err)
				}
				if diff := cmp.Diff(string(expected), renderedTemplate); diff != "" {
					t.Errorf("RenderTemplate() mismatch (-want +got):\n%s", diff)
				}
			}
		})
	}
}
