# cloudflared-fips Deployment Templates

Infrastructure-as-Code templates for deploying cloudflared-fips in AWS GovCloud.

## Terraform

```bash
cd terraform
cp variables.tfvars.example terraform.tfvars
# Edit terraform.tfvars with your values
terraform init
terraform plan
terraform apply
```

## CloudFormation

```bash
aws cloudformation deploy \
  --template-file cloudformation/cloudflared-fips.yaml \
  --stack-name cloudflared-fips \
  --parameter-overrides \
    TunnelToken=YOUR_TOKEN \
    ContainerImage=your-registry/cloudflared-fips:latest \
  --capabilities CAPABILITY_NAMED_IAM \
  --region us-gov-west-1
```

## Architecture

Both templates deploy:
- VPC with public/private subnets across 2 AZs
- NAT Gateway for outbound connectivity
- ECS Fargate cluster running cloudflared-fips containers
- Secrets Manager for tunnel credentials
- CloudWatch logging with 90-day retention
- Security groups allowing only Cloudflare edge traffic (443/tcp, 7844/udp)
- FIPS self-test as container health check

## GovCloud Notes

- Use `us-gov-west-1` or `us-gov-east-1` regions
- IAM ARNs use `arn:aws-us-gov:` partition
- FIPS endpoints are used by default in GovCloud
- Container images must be in a GovCloud-accessible registry (ECR in GovCloud)
