<template>
  <div class="page">
    <h1>{{ isEdit ? (isDraft ? '编辑草稿' : '编辑品鉴笔记') : '创建品鉴笔记' }}</h1>
    <el-form label-width="90px" class="form">
      <el-form-item label="豆种">
        <el-select v-model="beanId" placeholder="选择豆种自动填充" filterable clearable style="width: 320px" @change="onBeanChange">
          <el-option v-for="b in beans" :key="b.id" :label="`${b.name}（${b.origin}）`" :value="b.id" />
        </el-select>
      </el-form-item>
      <el-form-item label="咖啡名称"><el-input v-model="form.coffee_name" placeholder="如：埃塞俄比亚耶加雪菲（发布必填）" /></el-form-item>
      <el-form-item label="产地"><el-input v-model="form.origin" /></el-form-item>
      <el-form-item label="烘焙度">
        <el-radio-group v-model="form.roast_level">
          <el-radio-button v-for="(label, value) in RoastLevelMap" :key="value" :value="value">{{ label }}</el-radio-button>
        </el-radio-group>
      </el-form-item>
      <el-form-item label="风味标签"><FlavorTags :tags="form.flavor_tags" /></el-form-item>
      <el-form-item label="风味"><el-select v-model="tagInput" filterable allow-create default-first-option multiple style="width: 400px" placeholder="输入风味并回车" /></el-form-item>
      <el-form-item label="香气分"><el-rate v-model="form.aroma_score" :max="10" show-score /></el-form-item>
      <el-form-item label="酸质分"><el-rate v-model="form.acidity_score" :max="10" show-score /></el-form-item>
      <el-form-item label="醇厚度"><el-rate v-model="form.body_score" :max="10" show-score /></el-form-item>
      <el-form-item label="综合分"><el-rate v-model="form.overall_score" :max="10" show-score /></el-form-item>
      <el-form-item label="冲煮方式"><el-input v-model="form.brew_method" placeholder="如：手冲" /></el-form-item>
      <el-form-item label="关联配方">
        <el-select v-model="form.brew_recipe_id" clearable placeholder="选择冲煮配方" style="width: 320px">
          <el-option v-for="r in recipes" :key="r.id" :label="`${r.name}（${r.device}）`" :value="r.id" />
        </el-select>
      </el-form-item>
      <el-form-item label="品鉴笔记"><el-input v-model="form.notes_text" type="textarea" :rows="4" /></el-form-item>
      <el-form-item label="配图"><ImageUploader v-model="form.image_url" /></el-form-item>
      <el-form-item>
        <el-button type="primary" :loading="submitting" @click="submit('published')">发布笔记</el-button>
        <el-button :loading="savingDraft" @click="submit('draft')" :disabled="isPublished">保存草稿</el-button>
        <span v-if="isPublished" class="hint">已发布的笔记不能退回草稿</span>
      </el-form-item>
    </el-form>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { useRoute, useRouter } from 'vue-router'
import FlavorTags from '@/components/common/FlavorTags.vue'
import ImageUploader from '@/components/common/ImageUploader.vue'
import { createNote, updateNote, getNote } from '@/api/note'
import { listBeans } from '@/api/bean'
import { listRecipes } from '@/api/recipe'
import { useAuth } from '@/hooks/useAuth'
import { useUserStore } from '@/stores/useUserStore'
import { RoastLevelMap, parseTags, type RoastLevel, type TastingNote } from '@/constants/note'
import type { CoffeeBean } from '@/constants/bean'
import type { BrewRecipe } from '@/types/api'

const route = useRoute()
const router = useRouter()
const { user } = useAuth()
const userStore = useUserStore()
const beans = ref<CoffeeBean[]>([])
const recipes = ref<BrewRecipe[]>([])
const beanId = ref<number>()
const tagInput = ref<string[]>([])
const submitting = ref(false)
const savingDraft = ref(false)
const editing = ref<TastingNote | null>(null)

const isEdit = computed(() => !!route.params.id)
const isDraft = computed(() => editing.value?.status === 'draft')
const isPublished = computed(() => editing.value?.status === 'published')

const form = reactive({
  coffee_name: '', origin: '', roast_level: 'light' as string, flavor_tags: '[]',
  aroma_score: 0, acidity_score: 0, body_score: 0, overall_score: 0,
  brew_method: '', brew_recipe_id: 0, notes_text: '', image_url: '',
})

watch(tagInput, (v) => {
  form.flavor_tags = JSON.stringify(v)
})

onMounted(async () => {
  beans.value = (await listBeans({ page_size: 100 })).list
  recipes.value = (await listRecipes({ page_size: 100 })).list
  if (isEdit.value) {
    await userStore.ready
    await loadDraft(Number(route.params.id))
  }
})

async function loadDraft(id: number) {
  let res
  try {
    res = await getNote(id)
  } catch (e: any) {
    // Non-owners (and anonymous users) get 404: drafts behave as non-existent.
    if (e?.response?.status === 404) router.replace('/')
    return
  }
  const n = res.note
  if (n.status === 'draft' && user.value?.id !== n.user_id) {
    router.replace('/')
    return
  }
  editing.value = n
  Object.assign(form, {
    coffee_name: n.coffee_name,
    origin: n.origin,
    roast_level: n.roast_level,
    flavor_tags: n.flavor_tags || '[]',
    aroma_score: n.aroma_score,
    acidity_score: n.acidity_score,
    body_score: n.body_score,
    overall_score: n.overall_score,
    brew_method: n.brew_method,
    brew_recipe_id: n.brew_recipe_id || 0,
    notes_text: n.notes_text,
    image_url: n.image_url,
  })
  tagInput.value = parseTags(n.flavor_tags)
}

function onBeanChange(id: number | undefined) {
  const b = beans.value.find((x) => x.id === id)
  if (!b) return
  form.coffee_name = b.name
  form.origin = b.origin
  tagInput.value = parseTags(b.flavor_tags)
  form.flavor_tags = b.flavor_tags
}

function payload(status: 'draft' | 'published') {
  return {
    ...form,
    status,
    roast_level: form.roast_level as RoastLevel,
    brew_recipe_id: form.brew_recipe_id || 0,
  }
}

async function submit(status: 'draft' | 'published') {
  if (status === 'published' && !form.coffee_name.trim()) {
    ElMessage.warning('请填写咖啡名称后再发布')
    return
  }
  const loading = status === 'published' ? submitting : savingDraft
  loading.value = true
  try {
    let note: TastingNote
    if (isEdit.value) {
      // Published notes must not revert to draft, so omit status when editing
      // an already published note.
      const data = isPublished.value
        ? { ...payload('published'), status: undefined }
        : payload(status)
      note = await updateNote(Number(route.params.id), data)
    } else {
      note = await createNote(payload(status))
    }
    if (note.status === 'draft') {
      ElMessage.success('草稿已保存，可稍后继续编辑发布')
      editing.value = note
      router.replace(`/note/${note.id}/edit`)
    } else {
      ElMessage.success('品鉴笔记已发布')
      router.push(`/note/${note.id}`)
    }
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.page { max-width: 720px; margin: 0 auto; }
.form { margin-top: 16px; }
.hint { color: #999; font-size: 12px; margin-left: 8px; }
</style>
