package reporter

import (
	"encoding/xml"
	"fmt"
	"io"

	"github.com/catdevman/hatchet/pkg/hatchet"
)

// JUnit writes JUnit XML: one testsuite per target, one failing testcase per
// issue, a single passing testcase for clean targets, and an <error> case
// when the target itself could not be checked.
type JUnit struct{}

type junitSuites struct {
	XMLName  xml.Name     `xml:"testsuites"`
	Name     string       `xml:"name,attr"`
	Tests    int          `xml:"tests,attr"`
	Failures int          `xml:"failures,attr"`
	Errors   int          `xml:"errors,attr"`
	Suites   []junitSuite `xml:"testsuite"`
}

type junitSuite struct {
	Name     string      `xml:"name,attr"`
	Tests    int         `xml:"tests,attr"`
	Failures int         `xml:"failures,attr"`
	Errors   int         `xml:"errors,attr"`
	Cases    []junitCase `xml:"testcase"`
}

type junitCase struct {
	Name      string        `xml:"name,attr"`
	ClassName string        `xml:"classname,attr"`
	Failure   *junitFailure `xml:"failure,omitempty"`
	Error     *junitFailure `xml:"error,omitempty"`
}

type junitFailure struct {
	Message string `xml:"message,attr"`
	Body    string `xml:",chardata"`
}

func (JUnit) Report(w io.Writer, results []hatchet.Result) error {
	doc := junitSuites{Name: "hatchet"}

	for _, r := range results {
		suite := junitSuite{Name: r.Target}

		switch {
		case r.Err != nil:
			suite.Errors = 1
			suite.Tests = 1
			suite.Cases = append(suite.Cases, junitCase{
				Name:      "check " + r.Target,
				ClassName: r.Target,
				Error:     &junitFailure{Message: r.Err.Error()},
			})
		case len(r.Issues) == 0:
			suite.Tests = 1
			suite.Cases = append(suite.Cases, junitCase{
				Name:      "accessibility check",
				ClassName: r.Target,
			})
		default:
			// Every reported issue is a failing case; whether the run as a
			// whole passes is the exit code's job, not the reporter's.
			for _, is := range r.Issues {
				suite.Tests++
				suite.Failures++
				suite.Cases = append(suite.Cases, junitCase{
					Name:      fmt.Sprintf("%s: %s", is.Code, is.Selector),
					ClassName: r.Target,
					Failure: &junitFailure{
						Message: is.Message,
						Body:    fmt.Sprintf("Type: %s\nSelector: %s\nContext: %s", is.Type, is.Selector, is.Context),
					},
				})
			}
		}

		doc.Tests += suite.Tests
		doc.Failures += suite.Failures
		doc.Errors += suite.Errors
		doc.Suites = append(doc.Suites, suite)
	}

	if _, err := io.WriteString(w, xml.Header); err != nil {
		return err
	}
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	if err := enc.Encode(doc); err != nil {
		return err
	}
	_, err := io.WriteString(w, "\n")
	return err
}
