data "hivemind_forager_base_images" "available" {}

output "base_images" {
  value = data.hivemind_forager_base_images.available.images[*].image_ref
}
