import random
from locust import HttpUser, FastHttpUser, task, between

SEEDED_IDS = [1, 2, 3]  # POST only succeeds on seeded IDs (update-only design)


# Swap the base class to compare: HttpUser vs FastHttpUser
#class ProductUser(HttpUser):
class ProductUser(FastHttpUser):
    # Each user waits 0.1-0.5s between requests (shorten/remove for stress tests)
    wait_time = between(0.1, 0.5)

    @task(3)  # 3:1 GET:POST ratio
    def get_product(self):
        pid = random.choice(SEEDED_IDS)
        self.client.get(f"/products/{pid}", name="GET /products/[id]")

    @task(1)
    def post_product_details(self):
        pid = random.choice(SEEDED_IDS)
        self.client.post(
            f"/products/{pid}/details",
            json={
                "product_id": pid,  # must match path ID, or server returns 400
                "sku": f"SKU-{pid}",
                "manufacturer": "Locust",
                "category_id": 10,
                "weight": random.randint(0, 5000),
                "some_other_id": 100,
            },
            name="POST /products/[id]/details",
        )
    @task(1)
    def get_missing_product(self):
        with self.client.get("/products/99999", name="GET /products/[missing]",
                             catch_response=True) as resp:
            if resp.status_code == 404:
                resp.success()
            else:
                resp.failure(f"Expected 404, got {resp.status_code}")