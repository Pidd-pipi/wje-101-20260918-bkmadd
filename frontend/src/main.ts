import { createApp } from 'vue'
import { createPinia } from 'pinia'
import ElementPlus from 'element-plus'
import 'element-plus/dist/index.css'
import zhCn from 'element-plus/es/locale/lang/zh-cn'
import App from './App.vue'
import router from './router'
import { useUserStore } from './stores/useUserStore'

const app = createApp(App)
app.use(createPinia())
app.use(router)
app.use(ElementPlus, { locale: zhCn })

// 刷新或重新登录后恢复当前用户信息（token 已持久化在 localStorage），
// 等待恢复完成再挂载，避免本人草稿箱等页面误判为游客。
const userStore = useUserStore()
userStore.hydrate().finally(() => app.mount('#app'))
