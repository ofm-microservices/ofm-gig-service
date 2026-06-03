package main

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/fx"
)

func TestMainCmd(t *testing.T) {
	t.Helper()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Gig Main Suite")
}

var _ = Describe("main", func() {
	It("builds the fx app", func() {
		app := newApp()
		Expect(app).NotTo(BeNil())
	})

	It("invokes the runner from main", func() {
		original := runApp
		called := false
		runApp = func(*fx.App) {
			called = true
		}
		DeferCleanup(func() { runApp = original })

		main()
		Expect(called).To(BeTrue())
	})
})
