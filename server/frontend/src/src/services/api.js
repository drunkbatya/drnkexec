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

export async function fetchHosts({ page, pageSize }) {
  const res = await protectedApi.get("/hosts", {
    params: {
      page,
      page_size: pageSize,
    },
  });
  return res.data;
}

export async function fetchChecks({ page, pageSize, hostName, checkName }) {
  const res = await protectedApi.get("/checks", {
    params: {
      page,
      page_size: pageSize,
      host_name: hostName,
      check_name: checkName,
    },
  });
  return res.data;
}

export async function triggerCheckNow(hostName, checkName) {
  await protectedApi.post("/check/now", {
    host_name: hostName,
    check_name: checkName,
  });
}
