import axios from "axios";
import router from "../router";

const protectedApi = axios.create({
  baseURL: "/api/v1/admin",
  headers: {
    "Content-Type": "application/json",
  },
  withCredentials: true,
});

protectedApi.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response && error.response.status === 403) {
      router.replace({ name: "login" });
    }
    return Promise.reject(error);
  }
);

const userApi = axios.create({
  baseURL: "/api/v1/user",
  headers: {
    "Content-Type": "application/json",
  },
  withCredentials: true,
});

export async function login(login, password) {
  const res = await userApi.post("/login", { login, password });
  return res.data;
}

export async function logout() {
  await userApi.post("/logout");
}
