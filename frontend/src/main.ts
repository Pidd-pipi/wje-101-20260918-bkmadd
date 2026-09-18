import { createApp } from 'vue'
import { createPinia } from 'pinia'
import ElementPlus from 'element-plus'
import 'element-plus/dist/index.css'
import zhCn from 'element-plus/es/locale/lang/zh-cn'
import App from './App.vue'
import router from './router'
import { useUserStore } from './stores/useUserStore'

const app = createApp(App)
const pinia = createPinia()
app.use(pinia)
app.use(router)
app.use(ElementPlus, { locale: zhCn })

// Restore the current user after a refresh / re-login so owner-only UI
// (draft box, draft editor redirect) keeps working, then mount.
useUserStore().fetchProfile().finally(() => app.mount('#app'))
