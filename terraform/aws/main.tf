data "aws_ami" "ubuntu" {
  most_recent = true
  owners      = ["099720109477"]

  filter {
    name   = "name"
    values = [var.image]
  }

  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }
}

resource "aws_instance" "this" {
  ami                         = data.aws_ami.ubuntu.id
  instance_type               = var.size
  key_name                    = aws_key_pair.this.key_name
  vpc_security_group_ids      = [aws_security_group.this.id]
  associate_public_ip_address = true

  metadata_options {
    http_tokens = "required"
  }

  # The 8 GB AWS default is unusably small.
  root_block_device {
    volume_size = 40
    volume_type = "gp3"
    encrypted   = true
  }

  tags = {
    Name = "interview-labs-${var.slug}-vm"
  }
}

resource "aws_key_pair" "this" {
  key_name   = "interview-labs-${var.slug}-key"
  public_key = var.ssh_public_key
}

# Disposable interview VM: ssh open to the world by design (access is gated
# by the session keypair and pinned host key), full egress for tooling installs.
#trivy:ignore:AWS-0107
#trivy:ignore:AWS-0104
resource "aws_security_group" "this" {
  name        = "interview-labs-${var.slug}-fw"
  description = "interview-labs ${var.slug}: ssh in, full egress"

  ingress {
    description      = "ssh from anywhere (session keypair + pinned host key)"
    protocol         = "tcp"
    from_port        = 22
    to_port          = 22
    cidr_blocks      = ["0.0.0.0/0"]
    ipv6_cidr_blocks = ["::/0"]
  }

  egress {
    description      = "full egress for interview tooling installs"
    protocol         = "-1"
    from_port        = 0
    to_port          = 0
    cidr_blocks      = ["0.0.0.0/0"]
    ipv6_cidr_blocks = ["::/0"]
  }

  tags = {
    Name = "interview-labs-${var.slug}-fw"
  }
}
