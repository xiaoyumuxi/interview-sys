import tempfile
import unittest
from pathlib import Path

from app.memory import MemoryCandidateCreate, MemoryReviewRequest, MemoryStore


class MemoryStoreTests(unittest.TestCase):
    def setUp(self) -> None:
        self.temp_dir = tempfile.TemporaryDirectory()
        self.store = MemoryStore(str(Path(self.temp_dir.name) / "memory.sqlite3"))

    def tearDown(self) -> None:
        self.temp_dir.cleanup()

    def test_pending_candidate_does_not_enter_profile(self) -> None:
        candidate = self.store.create_candidate(
            MemoryCandidateCreate(
                type="weak_point",
                topic="redis",
                content="Redis lock renewal is unclear",
                confidence=0.8,
            )
        )

        self.assertEqual(candidate["status"], "pending")
        profile = self.store.profile("local-user")
        self.assertEqual(profile["items"], [])
        self.assertEqual(self.store.search("local-user", "redis"), [])

    def test_approve_updates_profile_and_due_review(self) -> None:
        candidate = self.store.create_candidate(
            MemoryCandidateCreate(
                type="weak_point",
                topic="redis",
                content="Redis lock renewal is unclear",
                confidence=0.8,
            )
        )

        approved = self.store.approve(candidate["candidate_id"], MemoryReviewRequest(review_note="confirmed"))

        self.assertEqual(approved["status"], "approved")
        profile = self.store.profile("local-user")
        self.assertEqual(len(profile["items"]), 1)
        self.assertEqual(profile["items"][0]["candidate_id"], candidate["candidate_id"])
        due_reviews = self.store.due_reviews("local-user")
        self.assertEqual(len(due_reviews), 1)
        self.assertEqual(due_reviews[0]["candidate_id"], candidate["candidate_id"])

    def test_search_returns_context_score_for_approved_memory(self) -> None:
        candidate = self.store.create_candidate(
            MemoryCandidateCreate(
                type="strong_point",
                topic="mysql",
                content="Understands index selectivity",
                confidence=0.9,
            )
        )
        self.store.approve(candidate["candidate_id"], MemoryReviewRequest())

        results = self.store.search("local-user", "mysql")

        self.assertEqual(len(results), 1)
        self.assertEqual(results[0]["candidate"]["candidate_id"], candidate["candidate_id"])
        self.assertIn("task_match_score", results[0]["reasons"])
        self.assertGreater(results[0]["memory_context_score"], 0)


if __name__ == "__main__":
    unittest.main()
