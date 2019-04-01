package main

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/codepipeline"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/aws/aws-sdk-go/service/sns"
)

type params struct {
	Bucket                  string `json:"bucket"`
	KeyPrefix               string `json:"key_prefix"`
	NotificationSubject     string `json:"notification_subject"`
	NotificationSNSTopicARN string `json:"notification_sns_topic_arn"`
}

// HandleLambdaEvent is triggered by CodePipeline and copies the artifacts content to S3. It also
// gives the copied S3 objects the bucket-owner-full-control ACL. If configured, it sends an SNS
// notification.
func HandleLambdaEvent(event events.CodePipelineEvent) error {
	sess := session.Must(session.NewSession())

	cpSvc := &codepipelineService{
		CodePipeline: codepipeline.New(sess),
		JobID:        event.CodePipelineJob.ID,
	}

	artiS3Svc := s3.New(sess, aws.NewConfig().WithCredentials(credentials.NewStaticCredentials(
		event.CodePipelineJob.Data.ArtifactCredentials.AccessKeyID,
		event.CodePipelineJob.Data.ArtifactCredentials.SecretAccessKey,
		event.CodePipelineJob.Data.ArtifactCredentials.SessionToken,
	)))
	s3Svc := s3.New(sess)

	userparams := event.CodePipelineJob.Data.ActionConfiguration.Configuration.UserParameters
	if userparams == "" {
		return cpSvc.failJob(errors.New("missing user params"))
	}

	var p params
	err := json.NewDecoder(strings.NewReader(userparams)).Decode(&p)
	if err != nil {
		return cpSvc.failJob(err)
	}
	if p.Bucket == "" {
		return cpSvc.failJob(errors.New("missing 'bucket' in user params"))
	}
	if p.KeyPrefix == "" {
		return cpSvc.failJob(errors.New("missing 'key_prefix' in user params"))
	}
	log.Println("user params:", p)

	artis := event.CodePipelineJob.Data.InputArtifacts
	if len(artis) == 0 {
		return cpSvc.failJob(errors.New("missing source artifacts"))
	}
	arti := artis[0]
	if arti.Location.LocationType != "S3" {
		return cpSvc.failJob(errors.New("location type of first artifact is not of type S3"))
	}

	tmpfile, err := ioutil.TempFile("", "codepipeline")
	if err != nil {
		return cpSvc.failJob(err)
	}
	defer func() {
		tmpfile.Close()
		os.Remove(tmpfile.Name())
	}()

	artiLoc := arti.Location.S3Location
	downloader := s3manager.NewDownloaderWithClient(artiS3Svc)
	n, err := downloader.Download(tmpfile, &s3.GetObjectInput{
		Bucket: aws.String(artiLoc.BucketName),
		Key:    aws.String(artiLoc.ObjectKey),
	})
	if err != nil {
		return cpSvc.failJob(err)
	}
	tmpfile.Close()
	log.Println("downloaded artifact to temp file. bytes:", n)

	zr, err := zip.OpenReader(tmpfile.Name())
	if err != nil {
		return cpSvc.failJob(err)
	}
	defer zr.Close()

	var uploadedKeys []string
	uploader := s3manager.NewUploaderWithClient(s3Svc)
	for _, f := range zr.File {
		log.Println("zip file file:", f.Name)
		rc, err := f.Open()
		if err != nil {
			return cpSvc.failJob(err)
		}
		key := fmt.Sprintf("%s/%s", p.KeyPrefix, f.Name)
		uploadedKeys = append(uploadedKeys, key)
		_, err = uploader.Upload(&s3manager.UploadInput{
			ACL:    aws.String("bucket-owner-full-control"),
			Body:   rc,
			Bucket: aws.String(p.Bucket),
			Key:    aws.String(key),
		})
		if err != nil {
			return cpSvc.failJob(err)
		}
		rc.Close()
	}

	if p.NotificationSNSTopicARN != "" {
		if p.NotificationSubject == "" {
			p.NotificationSubject = "S3 copy completed"
		}
		msg := "CodePipeline job #" + event.CodePipelineJob.ID + " uploaded S3 keys:\n\n" +
			strings.Join(uploadedKeys, "\n")
		snsSvc := sns.New(sess)
		_, err := snsSvc.Publish(&sns.PublishInput{
			Message:  aws.String(msg),
			Subject:  aws.String(p.NotificationSubject),
			TopicArn: aws.String(p.NotificationSNSTopicARN),
		})
		if err != nil {
			return cpSvc.failJob(err)
		}
	}

	return cpSvc.successJob()
}

func main() {
	lambda.Start(HandleLambdaEvent)
}
