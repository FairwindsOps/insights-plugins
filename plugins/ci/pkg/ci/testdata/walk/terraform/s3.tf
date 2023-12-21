resource "aws_s3_bucket" "data_exchange" {
  bucket_prefix = var.name_prefix
}

resource "aws_s3_bucket_acl" "data_exchange" {
  bucket = aws_s3_bucket.data_exchange.id
  acl    = "private"
}
