import random
from locust import FastHttpUser, task, between

SEARCH_TERMS = [
    "electro", "book", "home", "toys", "sports",
    "garden", "alpha", "beta", "omega", "product",
]


class SearchUser(FastHttpUser):
    wait_time = between(0, 0.05)

    @task
    def search(self):
        q = random.choice(SEARCH_TERMS)
        self.client.get(f"/products/search?q={q}", name="GET /products/search")
