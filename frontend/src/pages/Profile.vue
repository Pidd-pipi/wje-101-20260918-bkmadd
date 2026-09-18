<template>
  <div class="page" v-if="data">
    <el-card class="head">
      <div class="user-line">
        <UserAvatar :name="data.user.username" :size="64" />
        <div>
          <h2>{{ data.user.username }} <el-tag size="small">{{ data.user.role === 'admin' ? '管理员' : '咖啡爱好者' }}</el-tag></h2>
          <p class="bio">{{ data.user.bio || '这个人很懒，什么都没写' }}</p>
        </div>
      </div>
      <div class="stats">
        <div class="stat"><b>{{ data.note_count }}</b><span>品鉴次数</span></div>
        <div class="stat"><b>{{ data.avg_score.toFixed(1) }}</b><span>平均分</span></div>
        <div class="stat"><b>{{ data.likes_received }}</b><span>收到点赞</span></div>
        <div class="stat"><b>{{ data.followers }}</b><span>粉丝</span></div>
        <div class="stat"><b>{{ data.following }}</b><span>关注</span></div>
      </div>
      <div class="origins" v-if="data.top_origins.length">
        最爱产地 TOP3：<el-tag v-for="o in data.top_origins" :key="o" size="small" class="origin-tag">{{ o }}</el-tag>
      </div>
      <el-button v-if="isLoggedIn && user?.id !== Number($route.params.id)" :type="following ? 'default' : 'primary'" @click="toggleFollow">
        {{ following ? '已关注' : '关注' }}
      </el-button>
    </el-card>

    <div class="history-head">
      <h3>{{ isOwner ? '我的笔记' : '品鉴历史' }}</h3>
      <el-radio-group v-if="isOwner" v-model="activeTab" @change="onTabChange">
        <el-radio-button value="published">已发布</el-radio-button>
        <el-radio-button value="draft">草稿箱{{ draftCount ? `（${draftCount}）` : '' }}</el-radio-button>
      </el-radio-group>
    </div>

    <!-- 已发布 -->
    <el-row v-if="activeTab === 'published'" :gutter="16">
      <el-col v-for="n in data.notes" :key="n.id" :xs="24" :sm="12" :md="8">
        <el-card class="note-card" shadow="hover" @click="$router.push(`/note/${n.id}`)">
          <h4>{{ n.coffee_name }}</h4>
          <div class="meta">{{ n.origin }} · {{ RoastLevelMap[n.roast_level as RoastLevel] }}</div>
          <ScoreStars :model-value="n.overall_score" />
        </el-card>
      </el-col>
    </el-row>
    <EmptyState v-if="activeTab === 'published' && !data.notes.length" description="暂无已发布笔记" />

    <!-- 草稿箱（仅作者本人） -->
    <template v-if="isOwner && activeTab === 'draft'">
      <el-row :gutter="16">
        <el-col v-for="n in drafts" :key="n.id" :xs="24" :sm="12" :md="8">
          <el-card class="note-card" shadow="hover" @click="$router.push(`/note/${n.id}/edit`)">
            <el-tag size="small" type="info" class="draft-flag">草稿</el-tag>
            <h4 class="draft-title">{{ n.coffee_name || '未命名草稿' }}</h4>
            <div class="meta">{{ formatDate(n.updated_at || n.created_at) }} 更新 · {{ n.notes_text ? n.notes_text.slice(0, 30) : '暂无正文' }}</div>
            <div class="draft-actions" @click.stop>
              <el-button size="small" type="primary" :loading="publishingId === n.id" @click="publishDraft(n)">发布</el-button>
              <el-button size="small" @click="$router.push(`/note/${n.id}/edit`)">继续编辑</el-button>
              <el-button size="small" type="danger" plain @click="removeDraft(n)">删除</el-button>
            </div>
          </el-card>
        </el-col>
      </el-row>
      <EmptyState v-if="!drafts.length" description="草稿箱是空的" action-text="写一篇笔记" @action="$router.push('/note/create')" />
    </template>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import UserAvatar from '@/components/common/UserAvatar.vue'
import ScoreStars from '@/components/common/ScoreStars.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import { getUserProfile, followUser, unfollowUser } from '@/api/user'
import { listMyNotes, publishNote, deleteNote } from '@/api/note'
import { useAuth } from '@/hooks/useAuth'
import { useUserStore } from '@/stores/useUserStore'
import { RoastLevelMap, type RoastLevel, type TastingNote } from '@/constants/note'
import type { ProfileData } from '@/api/user'
import { formatDate } from '@/utils/dateFormat'

const route = useRoute()
const router = useRouter()
const { isLoggedIn, user } = useAuth()
const userStore = useUserStore()
const data = ref<ProfileData | null>(null)
const following = ref(false)
const activeTab = ref<'published' | 'draft'>((route.query.tab as string) === 'draft' ? 'draft' : 'published')
const drafts = ref<TastingNote[]>([])
const publishingId = ref<number | null>(null)

const isOwner = ref(false)
const draftCount = ref(0)

async function loadProfile() {
  await userStore.hydrate()
  data.value = await getUserProfile(route.params.id as string)
  isOwner.value = !!user.value && user.value.id === data.value.user.id
  if (isOwner.value && (activeTab.value === 'draft' || draftCount.value === 0)) {
    await loadDrafts()
  }
}

async function loadDrafts() {
  drafts.value = await listMyNotes('draft')
  draftCount.value = drafts.value.length
}

onMounted(loadProfile)

watch(() => route.params.id, (id) => {
  if (id && data.value && String(data.value.user.id) !== String(id)) {
    activeTab.value = (route.query.tab as string) === 'draft' ? 'draft' : 'published'
    drafts.value = []
    draftCount.value = 0
    loadProfile()
  }
})

function onTabChange(tab: string | number) {
  activeTab.value = tab as 'published' | 'draft'
  router.replace({ query: { ...route.query, tab: activeTab.value } })
  if (activeTab.value === 'draft') loadDrafts()
}

async function publishDraft(n: TastingNote) {
  if (!n.coffee_name?.trim()) {
    ElMessage.warning('请先补全咖啡名称再发布')
    router.push(`/note/${n.id}/edit`)
    return
  }
  if (!RoastLevelMap[n.roast_level as RoastLevel]) {
    ElMessage.warning('请先选择烘焙度再发布')
    router.push(`/note/${n.id}/edit`)
    return
  }
  publishingId.value = n.id
  try {
    await publishNote(n.id)
    ElMessage.success('品鉴笔记已发布')
    await Promise.all([loadDrafts(), loadProfile()])
  } finally {
    publishingId.value = null
  }
}

async function removeDraft(n: TastingNote) {
  await ElMessageBox.confirm('确定删除这篇草稿吗？删除后不可恢复。', '提示', { type: 'warning' })
  await deleteNote(n.id)
  ElMessage.success('草稿已删除')
  drafts.value = drafts.value.filter((x) => x.id !== n.id)
  draftCount.value = drafts.value.length
}

async function toggleFollow() {
  if (!isLoggedIn.value) {
    ElMessage.warning('请先登录')
    return
  }
  if (following.value) {
    await unfollowUser(data.value!.user.id)
    following.value = false
    ElMessage.success('已取消关注')
  } else {
    await followUser(data.value!.user.id)
    following.value = true
    ElMessage.success('关注成功')
  }
}
</script>

<style scoped>
.page { max-width: 1000px; margin: 0 auto; }
.head { margin-bottom: 20px; }
.user-line { display: flex; gap: 16px; align-items: center; }
.bio { color: #999; }
.stats { display: flex; gap: 32px; margin: 16px 0; }
.stat { display: flex; flex-direction: column; }
.stat b { font-size: 22px; color: #7b4b2a; }
.stat span { color: #999; font-size: 12px; }
.origins { margin: 12px 0; }
.origin-tag { margin-right: 6px; }
.history-head { display: flex; align-items: center; justify-content: space-between; margin-bottom: 12px; }
.history-head h3 { margin: 0; }
.note-card { margin-bottom: 16px; cursor: pointer; }
.draft-title { color: #7b4b2a; }
.draft-flag { margin-bottom: 6px; }
.meta { color: #999; font-size: 12px; }
.draft-actions { margin-top: 10px; display: flex; gap: 8px; cursor: default; }
</style>
