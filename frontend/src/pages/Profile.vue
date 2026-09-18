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

    <el-tabs v-model="activeTab" class="tabs">
      <el-tab-pane name="published">
        <template #label>已发布<el-badge v-if="data.notes.length" :value="data.notes.length" class="tab-badge" /></template>
      </el-tab-pane>
      <el-tab-pane v-if="isOwnProfile" name="drafts">
        <template #label>草稿箱<el-badge v-if="drafts.length" :value="drafts.length" type="info" class="tab-badge" /></template>
      </el-tab-pane>
    </el-tabs>

    <el-row v-if="activeTab === 'published'" :gutter="16">
      <el-col v-for="n in data.notes" :key="n.id" :xs="24" :sm="12" :md="8">
        <el-card class="note-card" shadow="hover" @click="$router.push(`/note/${n.id}`)">
          <h4>{{ n.coffee_name }}</h4>
          <div class="meta">{{ n.origin }} · {{ RoastLevelMap[n.roast_level] }}</div>
          <ScoreStars :model-value="n.overall_score" />
        </el-card>
      </el-col>
    </el-row>
    <EmptyState v-if="activeTab === 'published' && !data.notes.length" description="暂无品鉴记录" />

    <template v-if="activeTab === 'drafts' && isOwnProfile">
      <el-row :gutter="16">
        <el-col v-for="n in drafts" :key="n.id" :xs="24" :sm="12" :md="8">
          <el-card class="note-card draft-card" shadow="hover">
            <div class="draft-head" @click="$router.push(`/note/${n.id}/edit`)">
              <el-tag size="small" type="info">草稿</el-tag>
              <h4>{{ n.coffee_name || '未命名笔记' }}</h4>
            </div>
            <div class="meta">{{ n.origin || '未填写产地' }} · {{ RoastLevelMap[n.roast_level] }}</div>
            <p class="draft-text">{{ n.notes_text || '还没有正文内容' }}</p>
            <div class="draft-actions">
              <el-button size="small" type="primary" @click="$router.push(`/note/${n.id}/edit`)">继续编辑</el-button>
              <el-button size="small" @click="publishDraft(n)">发布</el-button>
              <el-button size="small" type="danger" plain @click="removeDraft(n)">删除</el-button>
            </div>
          </el-card>
        </el-col>
      </el-row>
      <EmptyState v-if="!drafts.length" description="草稿箱是空的" action-text="去写一篇" @action="$router.push('/note/create')" />
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import UserAvatar from '@/components/common/UserAvatar.vue'
import ScoreStars from '@/components/common/ScoreStars.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import { getUserProfile, followUser, unfollowUser } from '@/api/user'
import { publishNote, deleteNote } from '@/api/note'
import { useAuth } from '@/hooks/useAuth'
import { useUserStore } from '@/stores/useUserStore'
import { RoastLevelMap, type TastingNote } from '@/constants/note'
import type { ProfileData } from '@/api/user'

const route = useRoute()
const { isLoggedIn, user } = useAuth()
const userStore = useUserStore()
const data = ref<ProfileData | null>(null)
const following = ref(false)
const activeTab = ref<'published' | 'drafts'>('published')

const isOwnProfile = computed(() => !!user.value && user.value.id === Number(route.params.id))
const drafts = computed<TastingNote[]>(() => data.value?.drafts ?? [])

onMounted(async () => {
  await userStore.ready
  data.value = await getUserProfile(route.params.id as string)
  if (route.query.tab === 'drafts' && isOwnProfile.value) {
    activeTab.value = 'drafts'
  }
})

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

async function publishDraft(n: TastingNote) {
  if (!n.coffee_name?.trim()) {
    ElMessage.warning('请先补全咖啡名称再发布')
    return
  }
  await publishNote(n.id)
  ElMessage.success('品鉴笔记已发布')
  await reload()
  activeTab.value = 'published'
}

async function removeDraft(n: TastingNote) {
  try {
    await ElMessageBox.confirm(`确定删除草稿「${n.coffee_name || '未命名笔记'}」吗？`, '删除草稿', { type: 'warning' })
  } catch {
    return
  }
  await deleteNote(n.id)
  ElMessage.success('草稿已删除')
  await reload()
}

async function reload() {
  data.value = await getUserProfile(route.params.id as string)
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
.tabs { margin-bottom: 8px; }
.tab-badge { margin-left: 6px; }
.note-card { margin-bottom: 16px; cursor: pointer; }
.draft-card { border-style: dashed; }
.draft-head { display: flex; align-items: center; gap: 8px; }
.draft-head h4 { margin: 0; }
.meta { color: #999; font-size: 12px; }
.draft-text { color: #999; font-size: 13px; margin: 8px 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.draft-actions { display: flex; gap: 8px; cursor: default; }
</style>
