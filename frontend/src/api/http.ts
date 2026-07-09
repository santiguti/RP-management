import axios from "axios"

export const apiClient = axios.create({
  baseURL: "/api/v1",
  withCredentials: true,
})

function readCookie(name: string): string | null {
  const m = document.cookie.match(new RegExp("(?:^|; )" + name + "=([^;]*)"))
  return m ? decodeURIComponent(m[1]) : null
}

const MUTATING = new Set(["post", "patch", "put", "delete"])
apiClient.interceptors.request.use((cfg) => {
  if (cfg.method && MUTATING.has(cfg.method.toLowerCase())) {
    const csrf = readCookie("rp_csrf")
    if (csrf) cfg.headers["X-CSRF-Token"] = csrf
  }
  return cfg
})
