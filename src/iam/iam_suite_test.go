package iam_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"testing"
)

func TestIam(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "IAM Suite")
}
