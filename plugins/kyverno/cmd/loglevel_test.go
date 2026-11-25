package main

import (
	"testing"

	"github.com/sirupsen/logrus"
)

func TestSetLogLevel(t *testing.T) {
	for _, tc := range []struct {
		env  string
		want logrus.Level
	}{
		{"", logrus.InfoLevel},
		{"debug", logrus.DebugLevel},
		{"warn", logrus.WarnLevel},
		{"error", logrus.ErrorLevel},
	} {
		t.Setenv("LOG_LEVEL", tc.env)
		logrus.SetLevel(logrus.InfoLevel)
		setLogLevel()
		if got := logrus.GetLevel(); got != tc.want {
			t.Errorf("LOG_LEVEL=%q: got %v, want %v", tc.env, got, tc.want)
		}
	}
}
