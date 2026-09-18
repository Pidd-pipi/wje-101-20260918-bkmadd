import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { login as apiLogin, register as apiRegister, getProfile } from '@/api/user'
import { clearToken, getToken, setToken } from '@/utils/storage'
import type { UserInfo } from '@/constants/user'

export const useUserStore = defineStore('user', () => {
  const token = ref<string>(getToken())
  const user = ref<UserInfo | null>(null)
  // 启动时依据持久化 token 恢复用户信息的完成状态
  const hydrated = ref(false)
  let hydratePromise: Promise<void> | null = null

  const isLoggedIn = computed(() => !!token.value)
  const isAdmin = computed(() => user.value?.role === 'admin')

  async function login(username: string, password: string) {
    const res = await apiLogin({ username, password })
    token.value = res.token
    user.value = res.user
    hydrated.value = true
    setToken(res.token)
  }

  async function register(payload: { username: string; email: string; password: string; bio?: string }) {
    const res = await apiRegister(payload)
    token.value = res.token
    user.value = res.user
    hydrated.value = true
    setToken(res.token)
  }

  async function fetchProfile() {
    if (!token.value) {
      hydrated.value = true
      return
    }
    user.value = await getProfile(true)
    hydrated.value = true
  }

  // 应用启动时调用，保证刷新后各页面能等待用户信息恢复
  function hydrate() {
    if (!hydratePromise) {
      hydratePromise = fetchProfile().catch(() => {}).finally(() => { hydrated.value = true })
    }
    return hydratePromise
  }

  function logout() {
    token.value = ''
    user.value = null
    clearToken()
  }

  return { token, user, hydrated, isLoggedIn, isAdmin, login, register, fetchProfile, hydrate, logout }
})
