# lambda-codepipeline-s3

This AWS Lambda function is triggered by AWS CodePipeline. It copies the artifacts content to S3. It
also gives the copied S3 objects the `bucket-owner-full-control` ACL.

Configure the "Invoke / Lambda Function" action with `UserParameters` such as

```
{"bucket": "my-bucket", "key_prefix": "my/key/prefix"}
```

## Testing

No tests yet

## Building

```rake build```

This command compiles a Linux binary `./handler`.

## Deploying

```rake deploy```

This command deploys the code to the already existing Lambda function `codepipeline-s3-packages`.
