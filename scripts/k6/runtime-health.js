import http from "k6/http";
import { check, sleep } from "k6";

const baseURL = __ENV.BASE_URL || "http://127.0.0.1:8090";

export const options = {
  vus: Number(__ENV.VUS || 10),
  duration: __ENV.DURATION || "30s",
  thresholds: {
    http_req_failed: ["rate<0.01"],
    http_req_duration: ["p(95)<200"],
    checks: ["rate>0.99"],
  },
};

export default function () {
  const response = http.get(`${baseURL}/healthz`, {
    tags: { endpoint: "python_runtime_healthz" },
  });

  check(response, {
    "healthz returns 200": (res) => res.status === 200,
    "healthz body is ok": (res) => {
      try {
        return JSON.parse(res.body).status === "ok";
      } catch (_) {
        return false;
      }
    },
  });

  sleep(1);
}
