<template>
  <div class="page">
    <el-page-header @back="$router.back()" :content="isEdit ? '编辑品鉴笔记' : '创建品鉴笔记'" />
    <el-result v-if="loadFailed" icon="warning" title="笔记不存在或无权编辑" sub-title="该笔记可能是他人的草稿，或已被删除。">
      <template #extra><el-button type="primary" @click="$router.push('/')">返回首页</el-button></template>
    </el-result>
    <template v-else>
    <el-alert v-if="isEdit && note?.status === 'draft'" title="草稿：仅你本人可见，发布前不会出现在首页、搜索和他人主页。" type="info" show-icon :closable="false" class="draft-tip" />
    <el-form label-width="90px" class="form">
      <el-form-item label="豆种">
        <el-select v-model="beanId" placeholder="选择豆种自动填充" filterable clearable style="width: 320px" @change="onBeanChange">
          <el-option v-for="b in beans" :key="b.id" :label="`${b.name}（${b.origin}）`" :value="b.id" />
        </el-select>
      </el-form-item>
      <el-form-item label="咖啡名称" required><el-input v-model="form.coffee_name" placeholder="如：埃塞俄比亚耶加雪菲（发布时必填）" /></el-form-item>
      <el-form-item label="产地"><el-input v-model="form.origin" /></el-form-item>
      <el-form-item label="烘焙度" required>
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
        <el-button v-if="!isEdit || note?.status === 'draft'" :loading="submitting" @click="submit('draft')">保存草稿</el-button>
        <el-button type="primary" :loading="submitting" @click="submit('published')">
          {{ isEdit && note?.status === 'published' ? '保存修改' : '发布笔记' }}
        </el-button>
      </el-form-item>
    </el-form>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { useRoute, useRouter } from 'vue-router'
import FlavorTags from '@/components/common/FlavorTags.vue'
import ImageUploader from '@/components/common/ImageUploader.vue'
import { createNote, updateNote, publishNote, getNote } from '@/api/note'
import { listBeans } from '@/api/bean'
import { listRecipes } from '@/api/recipe'
import { RoastLevelMap, parseTags, NoteStatusDraft, type NoteStatus, type RoastLevel, type TastingNote } from '@/constants/note'
import type { CoffeeBean } from '@/constants/bean'
import type { BrewRecipe } from '@/types/api'

const route = useRoute()
const router = useRouter()
const editId = ref<number | null>(route.name === 'noteEdit' ? Number(route.params.id) : null)
const isEdit = computed(() => editId.value !== null)
const note = ref<TastingNote | null>(null)
const beans = ref<CoffeeBean[]>([])
const recipes = ref<BrewRecipe[]>([])
const beanId = ref<number>()
const tagInput = ref<string[]>([])
const submitting = ref(false)
const loadFailed = ref(false)
const form = reactive({
  coffee_name: '', origin: '', roast_level: '' as RoastLevel | '', flavor_tags: '[]',
  aroma_score: 0, acidity_score: 0, body_score: 0, overall_score: 0,
  brew_method: '', brew_recipe_id: 0, notes_text: '', image_url: '',
})

watch(tagInput, (v) => {
  form.flavor_tags = JSON.stringify(v)
})

onMounted(async () => {
  const [beanPage, recipePage] = await Promise.all([
    listBeans({ page_size: 100 }),
    listRecipes({ page_size: 100 }),
  ])
  beans.value = beanPage.list
  recipes.value = recipePage.list
  if (isEdit.value) {
    try {
      const res = await getNote(editId.value!)
      note.value = res.note
      Object.assign(form, {
        coffee_name: res.note.coffee_name,
        origin: res.note.origin,
        roast_level: res.note.roast_level,
        flavor_tags: res.note.flavor_tags || '[]',
        aroma_score: res.note.aroma_score,
        acidity_score: res.note.acidity_score,
        body_score: res.note.body_score,
        overall_score: res.note.overall_score,
        brew_method: res.note.brew_method,
        brew_recipe_id: res.note.brew_recipe_id,
        notes_text: res.note.notes_text,
        image_url: res.note.image_url,
      })
      tagInput.value = parseTags(res.note.flavor_tags)
    } catch {
      loadFailed.value = true
    }
  }
})

function onBeanChange(id: number | undefined) {
  const b = beans.value.find((x) => x.id === id)
  if (!b) return
  form.coffee_name = b.name
  form.origin = b.origin
  tagInput.value = parseTags(b.flavor_tags)
  form.flavor_tags = b.flavor_tags
}

function buildPayload(status: NoteStatus) {
  return {
    ...form,
    roast_level: status === NoteStatusDraft ? form.roast_level : (form.roast_level as RoastLevel),
    brew_recipe_id: form.brew_recipe_id || 0,
  }
}

async function submit(status: NoteStatus) {
  if (status === 'published' && !form.coffee_name.trim()) {
    ElMessage.warning('发布前请填写咖啡名称')
    return
  }
  if (status === 'published' && !RoastLevelMap[form.roast_level as RoastLevel]) {
    ElMessage.warning('发布前请选择烘焙度')
    return
  }
  submitting.value = true
  try {
    if (!isEdit.value) {
      if (status === 'draft') {
        const created = await createNote({ ...buildPayload(status), status })
        ElMessage.success('草稿已保存')
        router.push(`/profile/${created.user_id}?tab=draft`)
      } else {
        const created = await createNote({ ...buildPayload(status), status })
        ElMessage.success('品鉴笔记已发布')
        router.push(`/note/${created.id}`)
      }
      return
    }
    // 编辑已有笔记：先保存内容，草稿点击发布时再切换状态（发布不可逆）
    await updateNote(editId.value!, buildPayload(status))
    if (status === 'published' && note.value?.status === NoteStatusDraft) {
      await publishNote(editId.value!)
      ElMessage.success('品鉴笔记已发布')
    } else {
      ElMessage.success(status === NoteStatusDraft ? '草稿已保存' : '品鉴笔记已更新')
    }
    router.push(`/note/${editId.value}`)
  } finally {
    submitting.value = false
  }
}
</script>

<style scoped>
.page { max-width: 720px; margin: 0 auto; }
.form { margin-top: 16px; }
.draft-tip { margin-top: 12px; }
</style>
