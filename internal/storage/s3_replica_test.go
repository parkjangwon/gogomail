package storage

import "testing"

func baseS3Opts() S3Options {
	return S3Options{
		Region:          "us-east-1",
		Bucket:          "gogomail-mail",
		AccessKeyID:     "AKIAEXAMPLE",
		SecretAccessKey: "secret",
	}
}

func TestS3ReplicaTargetConfigured(t *testing.T) {
	t.Parallel()

	opts := baseS3Opts()
	opts.ReplicaRegion = "us-west-2"
	opts.ReplicaBucket = "gogomail-mail-dr"

	store, err := NewS3Store(opts)
	if err != nil {
		t.Fatalf("NewS3Store: %v", err)
	}
	region, bucket, ok := store.ReplicaTarget()
	if !ok {
		t.Fatal("ReplicaTarget reported not configured")
	}
	if region != "us-west-2" || bucket != "gogomail-mail-dr" {
		t.Fatalf("ReplicaTarget = (%q, %q), want (us-west-2, gogomail-mail-dr)", region, bucket)
	}
}

func TestS3ReplicaTargetNotConfigured(t *testing.T) {
	t.Parallel()

	store, err := NewS3Store(baseS3Opts())
	if err != nil {
		t.Fatalf("NewS3Store: %v", err)
	}
	if _, _, ok := store.ReplicaTarget(); ok {
		t.Fatal("ReplicaTarget reported configured with no replica set")
	}
}

func TestS3ReplicaRejectsSameRegionAndBucket(t *testing.T) {
	t.Parallel()

	opts := baseS3Opts()
	opts.ReplicaRegion = opts.Region
	opts.ReplicaBucket = opts.Bucket

	if _, err := NewS3Store(opts); err == nil {
		t.Fatal("NewS3Store accepted replica identical to primary region+bucket")
	}
}

func TestS3ReplicaRejectsInvalidBucket(t *testing.T) {
	t.Parallel()

	opts := baseS3Opts()
	opts.ReplicaBucket = "Invalid_Bucket_NAME"

	if _, err := NewS3Store(opts); err == nil {
		t.Fatal("NewS3Store accepted invalid replica bucket name")
	}
}
