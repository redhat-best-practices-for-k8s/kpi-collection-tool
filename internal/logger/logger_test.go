package logger

import (
	"log"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("InitLogger", func() {
	var tmpFile string

	BeforeEach(func() {
		tmpFile = "test_log.log"
	})

	AfterEach(func() {
		_ = os.Remove(tmpFile)
	})

	It("should create the log file and configure the logger to write to it", func() {
		f, err := InitLogger(tmpFile)
		Expect(err).NotTo(HaveOccurred())
		defer func() { _ = f.Close() }()

		info, err := os.Stat(tmpFile)
		Expect(err).NotTo(HaveOccurred())
		Expect(info.Size()).To(BeZero())

		log.Println("test log entry")

		content, err := os.ReadFile(tmpFile)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(content)).To(ContainSubstring("test log entry"))
	})
})
