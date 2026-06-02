package gotenbergtelegram

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
)

type UnitTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
}

func TestUnitTestSuite(t *testing.T) {
	suite.Run(t, new(UnitTestSuite))
}

func (s *UnitTestSuite) Test_DocumentGenerationWorkflow_Success() {
	env := s.NewTestWorkflowEnvironment()

	// Initialize dummy activities to mock them
	var activities *Activities

	// Mock ConvertHTMLToPDF activity
	dummyPdfBytes := []byte("%PDF-1.4 mock pdf data")
	env.OnActivity(activities.ConvertHTMLToPDF, mock.Anything, "<html>hello</html>").
		Return(dummyPdfBytes, nil)

	// Mock SendTelegramDocument activity
	env.OnActivity(activities.SendTelegramDocument, mock.Anything, int64(123456), dummyPdfBytes, "test.pdf").
		Return(nil)

	// Execute Workflow
	env.ExecuteWorkflow(DocumentGenerationWorkflow, "<html>hello</html>", int64(123456), "test.pdf")

	s.True(env.IsWorkflowCompleted())
	s.NoError(env.GetWorkflowError())
}

func (s *UnitTestSuite) Test_DocumentGenerationWorkflow_ConvertError() {
	env := s.NewTestWorkflowEnvironment()

	var activities *Activities

	// Mock ConvertHTMLToPDF to return conversion error
	env.OnActivity(activities.ConvertHTMLToPDF, mock.Anything, mock.Anything).
		Return(nil, context.DeadlineExceeded)

	env.ExecuteWorkflow(DocumentGenerationWorkflow, "<html>hello</html>", int64(123456), "test.pdf")

	s.True(env.IsWorkflowCompleted())
	s.Error(env.GetWorkflowError())
}
