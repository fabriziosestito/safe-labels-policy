package main

import (
	"github.com/deckarep/golang-set"
	"github.com/kubewarden/gjson"
	kubewarden "github.com/kubewarden/policy-sdk-go"

	"fmt"
	"regexp"
	"strings"
)

// A wrapper around the standard regexp.Regexp struct
// that implements marshalling and unmarshalling
type RegularExpression struct {
	*regexp.Regexp
}

// Convenience method to build a regular expression
func CompileRegularExpression(expr string) (*RegularExpression, error) {
	nativeRegExp, err := regexp.Compile(expr)
	if err != nil {
		return nil, err
	}
	return &RegularExpression{nativeRegExp}, nil
}

// UnmarshalText satisfies the encoding.TextMarshaler interface,
// also used by json.Unmarshal.
func (r *RegularExpression) UnmarshalText(text []byte) error {
	nativeRegExp, err := regexp.Compile(string(text))
	if err != nil {
		return err
	}
	r.Regexp = nativeRegExp
	return nil
}

// MarshalText satisfies the encoding.TextMarshaler interface,
// also used by json.Marshal.
func (r *RegularExpression) MarshalText() ([]byte, error) {
	if r.Regexp != nil {
		return []byte(r.Regexp.String()), nil
	}

	return nil, nil
}

type Settings struct {
	DeniedLabels      mapset.Set                    `json:"denied_labels"`
	MandatoryLabels   mapset.Set                    `json:"mandatory_labels"`
	ConstrainedLabels map[string]*RegularExpression `json:"constrained_labels"`
}

// Builds a new Settings instance starting from a validation
// request payload:
// {
//    "request": ...,
//    "settings": {
//       "denied_labels": [...],
//       "mandatory_labels": [...],
//       "constrained_labels": { ... }
//    }
// }
func NewSettingsFromValidationReq(payload []byte) (Settings, error) {
	// Note well: we don't validate the input JSON now, this has
	// already done inside of the `validate` function

	return newSettings(
		payload,
		"settings.denied_labels",
		"settings.mandatory_labels",
		"settings.constrained_labels")
}

// Builds a new Settings instance starting from a Settings
// payload:
// {
//    "denied_names": [ ... ],
//    "constrained_labels": { ... }
// }
func NewSettingsFromValidateSettingsPayload(payload []byte) (Settings, error) {
	if !gjson.ValidBytes(payload) {
		return Settings{}, fmt.Errorf("denied JSON payload")
	}

	return newSettings(
		payload,
		"denied_labels",
		"mandatory_labels",
		"constrained_labels")
}

func newSettings(payload []byte, paths ...string) (Settings, error) {
	if len(paths) != 3 {
		return Settings{}, fmt.Errorf("wrong number of json paths")
	}

	data := gjson.GetManyBytes(payload, paths...)

	deniedLabels := mapset.NewThreadUnsafeSet()
	data[0].ForEach(func(_, entry gjson.Result) bool {
		deniedLabels.Add(entry.String())
		return true
	})

	mandatoryLabels := mapset.NewThreadUnsafeSet()
	data[1].ForEach(func(_, entry gjson.Result) bool {
		mandatoryLabels.Add(entry.String())
		return true
	})

	constrainedLabels := make(map[string]*RegularExpression)
	var err error
	data[2].ForEach(func(key, value gjson.Result) bool {
		var regExp *RegularExpression
		regExp, err = CompileRegularExpression(value.String())
		if err != nil {
			return false
		}

		constrainedLabels[key.String()] = regExp
		return true
	})
	if err != nil {
		return Settings{}, err
	}

	return Settings{
		DeniedLabels:      deniedLabels,
		MandatoryLabels:   mandatoryLabels,
		ConstrainedLabels: constrainedLabels,
	}, nil
}

func (s *Settings) Valid() (bool, error) {
	constrainedLabels := mapset.NewThreadUnsafeSet()

	for label := range s.ConstrainedLabels {
		constrainedLabels.Add(label)
	}

	errors := []string{}

	constrainedAndDenied := constrainedLabels.Intersect(s.DeniedLabels)
	if constrainedAndDenied.Cardinality() != 0 {
		violations := []string{}
		for _, v := range constrainedAndDenied.ToSlice() {
			violations = append(violations, v.(string))
		}
		errors = append(
			errors,
			fmt.Sprintf(
				"These labels cannot be constrained and denied at the same time: %s",
				strings.Join(violations, ","),
			),
		)
	}

	mandatoryAndDenied := s.MandatoryLabels.Intersect(s.DeniedLabels)
	if mandatoryAndDenied.Cardinality() != 0 {
		violations := []string{}
		for _, v := range mandatoryAndDenied.ToSlice() {
			violations = append(violations, v.(string))
		}
		errors = append(
			errors,
			fmt.Sprintf(
				"These labels cannot be mandatory and denied at the same time: %s",
				strings.Join(violations, ","),
			),
		)
	}

	if len(errors) > 0 {
		return false, fmt.Errorf("%s", strings.Join(errors, "; "))
	}
	return true, nil
}

func validateSettings(payload []byte) ([]byte, error) {
	settings, err := NewSettingsFromValidateSettingsPayload(payload)
	if err != nil {
		// this happens when one of the user-defined regular expressions are invalid
		return kubewarden.RejectSettings(
			kubewarden.Message(fmt.Sprintf("Provided settings are not valid: %v", err)))
	}

	valid, err := settings.Valid()
	if valid {
		return kubewarden.AcceptSettings()
	}
	return kubewarden.RejectSettings(
		kubewarden.Message(fmt.Sprintf("Provided settings are not valid: %v", err)))
}
