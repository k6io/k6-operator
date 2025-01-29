package segmentation_test

import (
	"fmt"
	"testing"

	"github.com/grafana/k6-operator/pkg/segmentation"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestSegmentation(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Segmentation Suite")
}

var _ = Describe("the execution segmentation string generator", func() {
	When("given the index 1 and total 4", func() {
		It("should return proper segmentation fragments", func() {
			output, err := segmentation.NewCommandFragments(1, 4)
			fmt.Print(output)
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(Equal([]string{
				"--execution-segment=0:1/4",
				"--execution-segment-sequence=0,1/4,2/4,3/4,1",
			}))
		})
	})
})
