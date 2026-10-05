data "aws_availability_zones" "available" {
  state = "available"
}

locals {
  azs = slice(data.aws_availability_zones.available.names, 0, 2)
}

resource "aws_vpc" "main" {
  cidr_block           = var.vpc_cidr
  enable_dns_support   = true
  enable_dns_hostnames = true

  tags = { Name = "${var.project}-${var.environment}" }
}

# Sin reglas: nada usa el security group por defecto.
resource "aws_default_security_group" "default" {
  vpc_id = aws_vpc.main.id
}

# El dominio de busqueda del DHCP es el namespace de Cloud Map: dentro de la VPC
# "api" resuelve como api.<internal_domain>, igual que en la red del compose.
resource "aws_vpc_dhcp_options" "main" {
  domain_name         = var.internal_domain
  domain_name_servers = ["AmazonProvidedDNS"]
}

resource "aws_vpc_dhcp_options_association" "main" {
  vpc_id          = aws_vpc.main.id
  dhcp_options_id = aws_vpc_dhcp_options.main.id
}

# Subredes publicas: solo el ALB y la NAT.
resource "aws_subnet" "public" {
  count = length(local.azs)

  vpc_id                  = aws_vpc.main.id
  availability_zone       = local.azs[count.index]
  cidr_block              = cidrsubnet(var.vpc_cidr, 8, count.index)
  map_public_ip_on_launch = false

  tags = { Name = "${var.project}-public-${local.azs[count.index]}" }
}

# Subredes privadas: ECS y RDS.
resource "aws_subnet" "private" {
  count = length(local.azs)

  vpc_id            = aws_vpc.main.id
  availability_zone = local.azs[count.index]
  cidr_block        = cidrsubnet(var.vpc_cidr, 8, count.index + 10)

  tags = { Name = "${var.project}-private-${local.azs[count.index]}" }
}

resource "aws_internet_gateway" "main" {
  vpc_id = aws_vpc.main.id
}

# Coste frente a disponibilidad: con nat_gateway_per_az = true hay una NAT (y una
# tabla de rutas privada) por zona, de modo que la caida de una zona no deja sin
# salida a Internet a las tareas de la otra. Con false queda una sola NAT, mas
# barata pero con un punto unico de fallo. Las tareas la usan para bajar
# imagenes de Docker Hub.
locals {
  nat_count = var.nat_gateway_per_az ? length(local.azs) : 1
}

resource "aws_eip" "nat" {
  count = local.nat_count

  domain = "vpc"
}

resource "aws_nat_gateway" "main" {
  count = local.nat_count

  allocation_id = aws_eip.nat[count.index].id
  subnet_id     = aws_subnet.public[count.index].id

  depends_on = [aws_internet_gateway.main]
}

resource "aws_route_table" "public" {
  vpc_id = aws_vpc.main.id

  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.main.id
  }
}

resource "aws_route_table" "private" {
  count = local.nat_count

  vpc_id = aws_vpc.main.id

  route {
    cidr_block     = "0.0.0.0/0"
    nat_gateway_id = aws_nat_gateway.main[count.index].id
  }
}

resource "aws_route_table_association" "public" {
  count = length(local.azs)

  subnet_id      = aws_subnet.public[count.index].id
  route_table_id = aws_route_table.public.id
}

resource "aws_route_table_association" "private" {
  count = length(local.azs)

  subnet_id      = aws_subnet.private[count.index].id
  route_table_id = aws_route_table.private[var.nat_gateway_per_az ? count.index : 0].id
}
