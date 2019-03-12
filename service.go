package main

import (
	"log"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/codepipeline"
)

type codepipelineService struct {
	*codepipeline.CodePipeline
	JobID string
}

func (s *codepipelineService) successJob() error {
	log.Println("Job succeeded")
	_, err := s.CodePipeline.PutJobSuccessResult(&codepipeline.PutJobSuccessResultInput{
		JobId: aws.String(s.JobID),
	})
	return err
}

func (s *codepipelineService) failJob(err error) error {
	log.Println("Job failed with", err)
	_, err2 := s.CodePipeline.PutJobFailureResult(&codepipeline.PutJobFailureResultInput{
		JobId: aws.String(s.JobID),
		FailureDetails: &codepipeline.FailureDetails{
			Type:    aws.String(codepipeline.FailureTypeJobFailed),
			Message: aws.String(err.Error()),
		},
	})
	return err2
}
