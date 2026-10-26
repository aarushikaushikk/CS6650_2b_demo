# HW6 Part III: Application Load Balancer in front of the ECS service.

# The ALB gets its own security group: HTTP 80 from anywhere.
resource "aws_security_group" "alb" {
  name        = "${var.service_name}-alb-sg"
  description = "Allow HTTP 80 to the ALB"
  vpc_id      = var.vpc_id

  ingress {
    from_port   = 80
    to_port     = 80
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
    description = "HTTP from anywhere"
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
    description = "Allow all outbound (ALB to tasks on 8080)"
  }
}

resource "aws_lb" "this" {
  name               = "${var.service_name}-alb"
  load_balancer_type = "application"
  internal           = false
  subnets            = var.subnet_ids
  security_groups    = [aws_security_group.alb.id]
}

# Target group exactly as the assignment specifies.
resource "aws_lb_target_group" "this" {
  name        = "${var.service_name}-tg"
  port        = var.container_port # 8080
  protocol    = "HTTP"
  target_type = "ip" # required for Fargate (awsvpc)
  vpc_id      = var.vpc_id

  # Not specified by the assignment; AWS default is 300 s.
  # 30 s makes stopped/scaled-in tasks leave the ALB quickly.
  deregistration_delay = var.deregistration_delay

  health_check {
    path                = "/health"
    protocol            = "HTTP"
    interval            = 30
    healthy_threshold   = 2
    unhealthy_threshold = 3 # AWS default; not specified by the assignment
    timeout             = 5
    matcher             = "200"
  }
}

resource "aws_lb_listener" "http" {
  load_balancer_arn = aws_lb.this.arn
  port              = 80
  protocol          = "HTTP"

  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.this.arn
  }
}
