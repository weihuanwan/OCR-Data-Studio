<template>
  <div class="preview-center">
    <!-- 顶部 Header -->
    <div class="center-header">
      <div class="source-tag">源文件</div>

      <div class="file-meta" v-if="file">
        <span class="meta-icon">📎</span>
        <span class="meta-name" :title="file.path">{{ file.name }}</span>
        <span class="meta-sep">|</span>
        <span class="meta-size">{{ file.size || '0KB' }}</span>
      </div>

      <div v-if="focusMode" class="focus-indicator">
        <span class="focus-tag">聚焦模式</span>
        <span class="focus-info">正在查看第 {{ focusedBlockIndex + 1 }} 个识别区域</span>
        <button class="exit-focus-btn" @click.stop="exitFocus" title="退出聚焦 (ESC)">
          <span>✕</span> 查看全部
        </button>
      </div>
    </div>

    <div class="preview-body">
      <!-- 左侧图片 -->
      <div class="preview-panel image-panel" ref="imagePanel">
        <template v-if="currentPage && currentPage.imagePath && !currentPage.error">
          <div class="image-viewport">
            <div class="image-wrapper" ref="imageWrapper">
              <img
                  :src="getImageUrl(currentPage.imagePath)"
                  ref="previewImg"
                  class="preview-image"
                  @load="onImageLoad"
                  draggable="false"
              />

              <div class="overlay" v-if="imageLoaded" ref="overlay">
                <div
                    v-for="(block, index) in currentPage.blocks"
                    :key="index"
                    :ref="'ocrBox-' + index"
                    class="ocr-box"
                    :class="{
                      active: selectedBlockIndex === index,
                      'focus-hidden': focusMode && focusedBlockIndex !== index,
                      'focus-visible': focusMode && focusedBlockIndex === index
                    }"
                    :style="getBoxStyle(block)"
                    @click.stop="onBoxClick(index)"
                    :title="block.text"
                />
              </div>
            </div>
          </div>
        </template>

        <div v-else class="empty-state">
          <div class="empty-icon">🖼</div>
          <div class="empty-text">
            {{ currentPage?.error ? '图片加载异常' : '暂无图片预览' }}
          </div>
        </div>
      </div>

      <!-- 右侧内容 -->
      <div class="preview-panel content-panel" ref="contentPanel">
        <div v-if="isParsing" class="parsing-state">
          <div class="parsing-dots">
            <div class="dot"></div>
            <div class="dot"></div>
            <div class="dot"></div>
          </div>
          <div class="parsing-text">正在加载 / 识别中，请稍候...</div>
        </div>

        <template v-else-if="currentPage">
          <div v-if="currentPage.error" class="error-state">
            <div class="error-icon">⚠️</div>
            <div class="error-text">{{ currentPage.error }}</div>
          </div>

          <div
              v-else-if="currentPage.blocks && currentPage.blocks.length"
              class="blocks-list"
              :class="{ 'focus-mode': focusMode }"
          >
            <div
                v-for="(block, index) in currentPage.blocks"
                :key="index"
                :ref="'blockItem-' + index"
                class="block-item"
                :class="{
                  active: selectedBlockIndex === index,
                  'focus-hidden': focusMode && focusedBlockIndex !== index,
                  'focus-visible': focusMode && focusedBlockIndex === index,
                  'is-dirty': !!drafts[index] || editingTable.blockIndex === index || editingTextIndex === index
                }"
                @click="onBlockClick(index)"
            >
              <div class="block-index">{{ index + 1 }}</div>

              <div class="block-content">
                <!-- ========== 表格 ========== -->
                <template v-if="isTableBlock(block)">
                  <!-- 预览 -->
                  <div v-if="editingTable.blockIndex !== index" class="table-preview-card">
                    <div class="table-preview-header">
                      <span class="table-icon">📊</span>
                      <span class="table-title">表格</span>
                      <span class="edit-hint">点击一次选中，再点击一次编辑</span>
                    </div>

                    <div class="table-preview-body">
                      <div class="raw-table-host" v-html="getTableHtml(block)"></div>
                    </div>
                  </div>

                  <!-- 编辑 -->
                  <div v-else class="table-edit-mode">
                    <div class="table-toolbar">
                      <div class="toolbar-left">
                        <span class="table-edit-title">表格编辑</span>
                        <span class="table-edit-desc">点击单元格即可自动选中并编辑</span>
                      </div>

                      <div class="toolbar-middle">
                        <div class="tool-group">
                          <span class="group-label">行列</span>
                          <button class="tb-btn" type="button" @click.stop="insertTableRow('above')">↑ 上方行</button>
                          <button class="tb-btn" type="button" @click.stop="insertTableRow('below')">↓ 下方行</button>
                          <button class="tb-btn" type="button" @click.stop="insertTableColumn('left')">← 左侧列</button>
                          <button class="tb-btn" type="button" @click.stop="insertTableColumn('right')">→ 右侧列</button>
                        </div>

                        <div class="toolbar-divider"></div>

                        <div class="tool-group">
                          <button
                              class="tb-btn danger"
                              type="button"
                              :disabled="canDeleteSelectedRow"
                              @click.stop="deleteSelectedTableRow"
                          >
                            − 删除行
                          </button>

                          <button
                              class="tb-btn danger"
                              type="button"
                              :disabled="canDeleteSelectedColumn"
                              @click.stop="deleteSelectedTableColumn"
                          >
                            − 删除列
                          </button>
                        </div>

                        <div class="toolbar-divider"></div>

                        <div class="tool-group span-group">
                          <span class="group-label">合并/拆分</span>
                          <div class="span-inputs">
                            <label class="span-label">占行:</label>
                            <input type="number" v-model.number="targetRowspan" min="1" class="split-input" @click.stop />

                            <label class="span-label">占列:</label>
                            <input type="number" v-model.number="targetColspan" min="1" class="split-input" @click.stop />

                            <button class="tb-btn primary" type="button" @click.stop="applyCellSpan">应用</button>
                          </div>
                        </div>
                      </div>

                      <div class="toolbar-right">
                        <button class="tb-btn cancel" type="button" @click.stop="cancelTableEdit">取消</button>
                        <button class="tb-btn save" type="button" @click.stop="saveTableEdit">保存</button>
                      </div>
                    </div>

                    <div class="table-edit-tip">
                      💡 提示：鼠标点击单元格即可进入编辑。在“合并/拆分”中输入目标占列/占行数并点击“应用”，即可实现多列合并或拆分。
                    </div>

                    <div
                        ref="tableEditorHost"
                        class="table-edit-area"
                        @click.stop="onTableEditorClick"
                        @focusin="onTableEditorFocusIn"
                        @input="onTableEditorInput"
                    ></div>
                  </div>
                </template>

                <!-- ========== 普通文本 ========== -->
                <template v-else>
                  <div class="text-card">
                    <div class="text-header">
                      <span class="text-icon">📝</span>
                      <span class="text-title">识别内容</span>
                      <span v-if="editingTextIndex === index" class="unsaved-badge">编辑中</span>
                    </div>

                    <textarea
                        :ref="'textInput-' + index"
                        class="block-text-input"
                        :class="{ 'is-editing': editingTextIndex === index }"
                        :value="getDisplayBlock(index).text"
                        @input="onTextInput(index, $event.target.value)"
                        @focus="$emit('select-block', index)"
                        @click.stop="onTextInputClick(index)"
                        spellcheck="false"
                        :readonly="editingTextIndex !== index"
                        rows="3"
                        placeholder="识别内容为空"
                    />

                    <div v-if="editingTextIndex === index" class="edit-actions">
                      <button class="save-btn" @click.stop="saveBlock(index)">
                        <span>✓</span> 保存
                      </button>

                      <button class="cancel-btn" @click.stop="cancelEdit(index)">
                        <span>✕</span> 取消
                      </button>
                    </div>
                  </div>
                </template>

                <div class="block-meta" v-if="block.score !== undefined">
                  置信度 {{ (block.score * 100).toFixed(1) }}%
                </div>
              </div>
            </div>
          </div>

          <div v-else class="empty-state">
            <div class="empty-icon">📄</div>
            <div class="empty-text">未识别到文本内容</div>
          </div>
        </template>

        <div v-else class="empty-state">
          <div class="empty-icon">📂</div>
          <div class="empty-text">请选择或导入文件</div>
        </div>
      </div>
    </div>

    <!-- 底部工具栏 -->
    <div class="preview-toolbar">
      <button class="tool-btn" :disabled="currentPageIndex <= 0" @click="prevPage" title="上一页">◀</button>

      <div class="page-input">
        <input type="text" :value="currentPageIndex + 1" @blur="onPageInput" @keyup.enter="onPageInput" />
        <span>/</span>
        <span>{{ totalPages }}</span>
      </div>

      <button class="tool-btn" :disabled="currentPageIndex >= totalPages - 1" @click="nextPage" title="下一页">▶</button>

      <div class="toolbar-sep" />

      <button class="tool-btn" @click="zoomOut" title="缩小">−</button>
      <span class="zoom-value">{{ Math.round(scale * 100) }}%</span>
      <button class="tool-btn" @click="zoomIn" title="放大">＋</button>
      <button class="tool-btn" @click="rotate" title="顺时针旋转">↻</button>
      <button class="tool-btn" @click="resetView" title="重置视图">⟲</button>
      <button class="tool-btn" @click="fitView" title="适配视图">⤢</button>

      <div class="toolbar-sep" />

      <div class="export-wrapper">
        <button class="tool-btn export-btn" @click.stop="toggleExportMenu" title="导出数据集 JSON">
          <span>📥</span>
          <span>导出数据集</span>
          <span class="export-arrow">▼</span>
        </button>

        <div v-if="showExportMenu" class="export-dropdown" @click.stop>
          <div class="dropdown-header">排除以下内容</div>

          <label class="dropdown-item" v-for="opt in labelOptions" :key="opt.value">
            <input type="checkbox" :value="opt.value" v-model="excludeLabels" />
            <span>{{ opt.label }}</span>
          </label>

          <div class="dropdown-footer">
            <button class="tb-btn" @click="excludeLabels = []">清空</button>
            <button class="tb-btn save" @click="confirmExport">确认导出</button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { ExportData } from "../../wailsjs/go/main/App"

export default {
  name: 'PreviewCenter',

  props: {
    file: { type: Object, default: null },
    pages: { type: Array, default: () => [] },
    currentPageIndex: { type: Number, default: 0 },
    selectedBlockIndex: { type: Number, default: -1 },
    isParsing: { type: Boolean, default: false }
  },

  emits: ['select-block', 'change-page', 'update-block'],

  data() {
    return {
      imageLoaded: false,
      imageSize: {
        width: 0,
        height: 0,
        naturalWidth: 1,
        naturalHeight: 1,
        offsetX: 0,
        offsetY: 0
      },
      scale: 1,
      rotation: 0,
      focusMode: false,
      focusedBlockIndex: -1,
      resizeObserver: null,
      wheelPanel: null,

      drafts: {},
      editingTextIndex: -1,
      editingTable: {
        blockIndex: -1,
        originalHtml: ''
      },
      editActiveCell: {
        rowIndex: -1,
        colIndex: -1
      },
      targetColspan: 1,
      targetRowspan: 1,

      showExportMenu: false,
      excludeLabels: [],
      labelOptions: [
        { label: '表格 (Table)', value: 'table' },
        { label: '公式 (Formula)', value: 'formula_number' },
        { label: '图表 (Chart)', value: 'chart' },
        { label: '印章 (Seal)', value: 'seal' },
        { label: '定位 (Spotting)', value: 'spotting' },
        { label: '普通文本 (OCR)', value: 'text' }
      ]
    }
  },

  computed: {
    currentPage() {
      if (!this.pages || this.pages.length === 0) return null
      return this.pages[this.currentPageIndex] || null
    },

    totalPages() {
      return this.pages?.length || 0
    },

    canDeleteSelectedRow() {
      const table = this.getEditorTable()
      debugger
      if (!table || table.rows.length <= 1) return false
      return this.editActiveCell.rowIndex >= 0
    },

    canDeleteSelectedColumn() {
      const table = this.getEditorTable()
      if (!table) return false
      return this.getTableColumnCount(table) > 1 && this.editActiveCell.colIndex >= 0
    }
  },

  watch: {
    currentPageIndex() {
      this.imageLoaded = false
      this.scale = 1
      this.rotation = 0
      this.drafts = {}
      this.editingTextIndex = -1
      this.cancelTableEdit()
      this.exitFocus()

      this.$nextTick(() => {
        this.applyTransform()
        this.centerImage()
      })
    },

    selectedBlockIndex(newVal) {
      if (newVal >= 0) {
        this.$nextTick(() => {
          this.scrollToBlock(newVal)
          this.scrollToBox(newVal)
        })
      }
    }
  },

  mounted() {
    document.addEventListener('keydown', this.handleKeydown)
    document.addEventListener('click', this.handleGlobalClick)

    this.wheelPanel = this.$refs.imagePanel
    if (this.wheelPanel) {
      this.wheelPanel.addEventListener('wheel', this.onWheel, { passive: false })
    }

    this.initResizeObserver()
  },

  beforeUnmount() {
    document.removeEventListener('keydown', this.handleKeydown)
    document.removeEventListener('click', this.handleGlobalClick)

    if (this.wheelPanel) {
      this.wheelPanel.removeEventListener('wheel', this.onWheel)
    }

    if (this.resizeObserver) {
      this.resizeObserver.disconnect()
    }
  },

  methods: {
    // ================= 导出菜单 =================
    toggleExportMenu() {
      this.showExportMenu = !this.showExportMenu
    },

    handleGlobalClick(e) {
      if (this.showExportMenu && !e.target.closest('.export-wrapper')) {
        this.showExportMenu = false
      }
    },

    confirmExport() {
      this.showExportMenu = false
      this.exportData()
    },

    exportData() {
      if (!this.pages || this.pages.length === 0) {
        console.warn('没有可导出的数据')
        return
      }

      ExportData(this.pages, this.excludeLabels)
          .then(result => {
            console.log('导出成功', result)
          })
          .catch(err => {
            console.error('导出失败', err)
          })
    },

    // ================= 图片基础 =================
    getImageUrl(path) {
      if (!path) return ''
      if (path.startsWith('data:') || path.startsWith('http')) {
        return path
      }
      return `http://wails.localhost${path}`
    },

    refreshImageMetrics() {
      const img = this.$refs.previewImg
      if (!img) return

      this.imageSize = {
        width: img.clientWidth || 1,
        height: img.clientHeight || 1,
        naturalWidth: img.naturalWidth || 1,
        naturalHeight: img.naturalHeight || 1,
        offsetX: img.offsetLeft || 0,
        offsetY: img.offsetTop || 0
      }
    },

    onImageLoad() {
      this.$nextTick(() => {
        requestAnimationFrame(() => {
          this.refreshImageMetrics()
          this.imageLoaded = true
          this.centerImage()

          if (this.resizeObserver && this.$refs.previewImg) {
            this.resizeObserver.disconnect()
            this.resizeObserver.observe(this.$refs.previewImg)
          }
        })
      })
    },

    initResizeObserver() {
      if (typeof ResizeObserver === 'undefined') return

      this.resizeObserver = new ResizeObserver(entries => {
        for (const entry of entries) {
          if (entry.target === this.$refs.previewImg && this.imageLoaded) {
            this.refreshImageMetrics()
          }
        }
      })

      if (this.$refs.previewImg) {
        this.resizeObserver.observe(this.$refs.previewImg)
      }
    },

    applyTransform() {
      const wrapper = this.$refs.imageWrapper
      if (wrapper) {
        wrapper.style.transform = `scale(${this.scale}) rotate(${this.rotation}deg)`
      }
    },

    centerImage() {
      const panel = this.$refs.imagePanel
      if (!panel) return

      panel.scrollTop = Math.max(0, (panel.scrollHeight - panel.clientHeight) / 2)
      panel.scrollLeft = Math.max(0, (panel.scrollWidth - panel.clientWidth) / 2)
    },

    resetView() {
      this.scale = 1
      this.rotation = 0
      this.applyTransform()

      this.$nextTick(() => {
        this.centerImage()
      })
    },

    fitView() {
      this.scale = 1
      this.applyTransform()

      this.$nextTick(() => {
        this.centerImage()
      })
    },

    setScale(nextScale) {
      this.scale = Math.min(4, Math.max(0.25, nextScale))
      this.applyTransform()
    },

    zoomIn() {
      this.setScale(this.scale + 0.25)
    },

    zoomOut() {
      this.setScale(this.scale - 0.25)
    },

    rotate() {
      this.rotation = (this.rotation + 90) % 360
      this.applyTransform()
    },

    onWheel(e) {
      if (!e.ctrlKey && !e.metaKey) return
      e.preventDefault()

      const step = e.deltaY > 0 ? -0.1 : 0.1
      this.setScale(this.scale + step)
    },

    getBoxStyle(block) {
      if (!this.imageLoaded || !block) return {}

      let points = []

      if (block.point && Array.isArray(block.point) && block.point.length >= 4) {
        if (block.point.length === 4) {
          const [x1, y1, x2, y2] = block.point
          points = [
            { x: x1, y: y1 },
            { x: x2, y: y1 },
            { x: x2, y: y2 },
            { x: x1, y: y2 }
          ]
        } else if (block.point.length % 2 === 0) {
          for (let i = 0; i < block.point.length; i += 2) {
            points.push({
              x: block.point[i],
              y: block.point[i + 1]
            })
          }
        }
      } else if (block.polygonPoints && Array.isArray(block.polygonPoints)) {
        points = block.polygonPoints.map(p => ({
          x: p.x ?? p.X,
          y: p.y ?? p.Y
        }))
      }

      if (points.length === 0) return {}

      const { width, height, naturalWidth, naturalHeight, offsetX, offsetY } = this.imageSize
      const scaleX = width / naturalWidth
      const scaleY = height / naturalHeight

      const xs = points.map(p => p.x)
      const ys = points.map(p => p.y)

      const minX = Math.min(...xs)
      const minY = Math.min(...ys)
      const maxX = Math.max(...xs)
      const maxY = Math.max(...ys)

      return {
        left: `${minX * scaleX + offsetX}px`,
        top: `${minY * scaleY + offsetY}px`,
        width: `${(maxX - minX) * scaleX}px`,
        height: `${(maxY - minY) * scaleY}px`
      }
    },

    scrollToBox(index) {
      const panel = this.$refs.imagePanel
      const boxRefs = this.$refs['ocrBox-' + index]
      const box = Array.isArray(boxRefs) ? boxRefs[0] : boxRefs

      if (!panel || !box) return

      const panelRect = panel.getBoundingClientRect()
      const boxRect = box.getBoundingClientRect()

      panel.scrollLeft += (boxRect.left + boxRect.width / 2) - (panelRect.left + panelRect.width / 2)
      panel.scrollTop += (boxRect.top + boxRect.height / 2) - (panelRect.top + panelRect.height / 2)
    },

    scrollToBlock(index) {
      const el = this.$refs['blockItem-' + index]
      const target = Array.isArray(el) ? el[0] : el
      if (target) {
        target.scrollIntoView({ behavior: 'smooth', block: 'center' })
      }
    },

    // ================= 页面/块选择 =================
    prevPage() {
      if (this.currentPageIndex > 0) {
        this.$emit('change-page', this.currentPageIndex - 1)
      }
    },

    nextPage() {
      if (this.currentPageIndex < this.totalPages - 1) {
        this.$emit('change-page', this.currentPageIndex + 1)
      }
    },

    onPageInput(e) {
      const val = parseInt(e.target.value, 10)
      if (!isNaN(val) && val >= 1 && val <= this.totalPages) {
        this.$emit('change-page', val - 1)
      } else {
        e.target.value = this.currentPageIndex + 1
      }
    },

    onBoxClick(index) {
      if (this.editingTable.blockIndex >= 0 || this.editingTextIndex >= 0) return
      this.exitFocus()
      this.$emit('select-block', index)
    },

    onBlockClick(index) {
      if (this.editingTable.blockIndex === index || this.editingTextIndex === index) return

      if (this.editingTable.blockIndex >= 0 && this.editingTable.blockIndex !== index) {
        this.cancelTableEdit()
      }

      if (this.editingTextIndex >= 0 && this.editingTextIndex !== index) {
        this.cancelEdit(this.editingTextIndex)
      }

      if (this.selectedBlockIndex === index) {
        this.enterEditMode(index)
      } else {
        this.exitFocus()
        this.$emit('select-block', index)
      }
    },

    onTextInputClick(index) {
      if (this.editingTextIndex !== index) {
        this.$emit('select-block', index)
      }
    },

    enterEditMode(index) {
      const block = this.currentPage?.blocks?.[index]
      if (!block) return

      this.$emit('select-block', index)
      this.enterFocus(index)

      if (this.isTableBlock(block)) {
        this.startTableEdit(index)
      } else {
        this.startTextEdit(index)
      }
    },

    enterFocus(index) {
      this.focusedBlockIndex = index
      this.focusMode = true
    },

    exitFocus() {
      this.focusMode = false
      this.focusedBlockIndex = -1
    },

    handleKeydown(e) {
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 's') {
        if (this.editingTable.blockIndex >= 0) {
          e.preventDefault()
          this.saveTableEdit()
        } else if (this.editingTextIndex >= 0) {
          e.preventDefault()
          this.saveBlock(this.editingTextIndex)
        }
        return
      }

      if (e.key === 'Escape') {
        if (this.editingTable.blockIndex >= 0) {
          this.cancelTableEdit()
        } else if (this.editingTextIndex >= 0) {
          this.cancelEdit(this.editingTextIndex)
        } else if (this.focusMode) {
          this.exitFocus()
        }
        return
      }

      const target = e.target
      const editable = target && (
          target.tagName === 'INPUT' ||
          target.tagName === 'TEXTAREA' ||
          target.tagName === 'SELECT' ||
          target.isContentEditable
      )

      if (!editable) {
        if (e.key === 'ArrowLeft') {
          this.prevPage()
        } else if (e.key === 'ArrowRight') {
          this.nextPage()
        } else if (e.key === 'ArrowUp') {
          e.preventDefault()
          this.selectNextBlock(-1)
        } else if (e.key === 'ArrowDown') {
          e.preventDefault()
          this.selectNextBlock(1)
        }
      }
    },

    selectNextBlock(delta) {
      const blocks = this.currentPage?.blocks
      if (!blocks || blocks.length === 0) return

      let idx = this.selectedBlockIndex
      if (idx < 0) {
        idx = delta > 0 ? 0 : blocks.length - 1
      } else {
        idx += delta
      }

      if (idx < 0 || idx >= blocks.length) return

      this.$emit('select-block', idx)
    },

    // ================= 普通文本编辑 =================
    getDisplayBlock(index) {
      const original = this.currentPage?.blocks?.[index] || {}
      const draft = this.drafts[index]
      return draft ? { ...original, text: draft.text } : original
    },

    onTextInput(index, value) {
      const block = this.currentPage.blocks[index]
      if (!this.drafts[index]) {
        this.drafts = {
          ...this.drafts,
          [index]: { text: block.text }
        }
      }

      this.drafts = {
        ...this.drafts,
        [index]: {
          ...this.drafts[index],
          text: value
        }
      }
    },

    startTextEdit(index) {
      if (this.editingTable.blockIndex >= 0) return

      this.editingTextIndex = index

      if (!this.drafts[index]) {
        const block = this.currentPage.blocks[index]
        this.drafts = {
          ...this.drafts,
          [index]: { text: block.text }
        }
      }

      this.$nextTick(() => {
        const el = this.$refs['textInput-' + index]
        const target = Array.isArray(el) ? el[0] : el
        if (target) target.focus()
      })
    },

    saveBlock(index) {
      const draft = this.drafts[index]
      const block = this.currentPage.blocks[index]

      if (draft && draft.text !== block.text) {
        this.$emit('update-block', {
          pageIndex: this.currentPageIndex,
          blockIndex: index,
          block: {
            ...block,
            text: draft.text
          }
        })
      }

      const newDrafts = { ...this.drafts }
      delete newDrafts[index]
      this.drafts = newDrafts

      this.editingTextIndex = -1
      this.exitFocus()

      this.$nextTick(() => {
        this.scrollToBlock(index)
      })
    },

    cancelEdit(index) {
      const newDrafts = { ...this.drafts }
      delete newDrafts[index]
      this.drafts = newDrafts

      this.editingTextIndex = -1
      this.exitFocus()

      if (this.selectedBlockIndex === index) {
        this.$nextTick(() => {
          this.scrollToBlock(index)
        })
      }
    },

    // ================= 表格编辑 =================
    startTableEdit(index) {
      if (this.editingTable.blockIndex >= 0) return

      if (this.editingTextIndex >= 0) {
        this.cancelEdit(this.editingTextIndex)
      }

      const block = this.currentPage?.blocks?.[index]
      if (!block) return

      const html = this.getTableHtml(block)
      this.editingTable = {
        blockIndex: index,
        originalHtml: html
      }

      this.editActiveCell = {
        rowIndex: -1,
        colIndex: -1
      }

      this.$nextTick(() => {
        const host = this.getTableEditorHost()
        if (!host) return

        host.innerHTML = html
        this.prepareEditableTable()

        const table = this.getEditorTable()
        const firstCell = table?.querySelector('td, th')
        if (firstCell) {
          this.selectTableCell(firstCell)
        }
      })
    },

    getTableEditorHost() {
      const ref = this.$refs.tableEditorHost
      return Array.isArray(ref) ? ref[0] : ref
    },

    getEditorTable() {
      return this.getTableEditorHost()?.querySelector('table') || null
    },

    prepareEditableTable() {
      const table = this.getEditorTable()
      if (!table) return

      table.classList.add('editable-table')
      table.setAttribute('data-editor-table', 'true')

      table.removeAttribute('border')
      table.removeAttribute('cellspacing')
      table.removeAttribute('cellpadding')
      table.removeAttribute('width')
      table.removeAttribute('height')
      table.style.removeProperty('border')

      table.querySelectorAll('td, th').forEach(cell => {
        cell.setAttribute('contenteditable', 'true')
        cell.setAttribute('spellcheck', 'false')
        cell.setAttribute('tabindex', '0')

        cell.style.removeProperty('border')
        cell.style.removeProperty('border-top')
        cell.style.removeProperty('border-right')
        cell.style.removeProperty('border-bottom')
        cell.style.removeProperty('border-left')
        cell.style.removeProperty('background-color')
        cell.style.removeProperty('width')
        cell.style.removeProperty('height')
      })
    },

    onTableEditorClick(e) {
      const cell = e.target.closest?.('td, th')
      if (!cell) return

      const table = this.getEditorTable()
      if (!table || !table.contains(cell)) return

      this.selectTableCell(cell)
    },

    onTableEditorFocusIn(e) {
      const cell = e.target.closest?.('td, th')
      if (!cell) return

      const table = this.getEditorTable()
      if (table && table.contains(cell)) {
        this.selectTableCell(cell)
      }
    },

    selectTableCell(cell) {
      const table = this.getEditorTable()
      if (!table || !cell) return

      table.querySelectorAll('.editor-selected-cell').forEach(el => {
        el.classList.remove('editor-selected-cell')
      })

      cell.classList.add('editor-selected-cell')
      this.editActiveCell = this.getCellPosition(table, cell)

      const span = this.getCellSpan(cell)
      this.targetColspan = span.colspan
      this.targetRowspan = span.rowspan
    },

    onTableEditorInput() {
      this.$nextTick(() => {
        this.prepareEditableTable()
      })
    },

    serializeEditorTable() {
      const table = this.getEditorTable()
      if (!table) return ''

      const copy = table.cloneNode(true)
      copy.removeAttribute('data-editor-table')
      copy.classList.remove('editable-table')

      copy.querySelectorAll('.editor-selected-cell').forEach(cell => {
        cell.classList.remove('editor-selected-cell')
      })

      copy.querySelectorAll('[contenteditable]').forEach(cell => {
        cell.removeAttribute('contenteditable')
        cell.removeAttribute('spellcheck')
        cell.removeAttribute('tabindex')
      })

      return copy.outerHTML
    },

    sanitizeTableHtml(html) {
      if (!html) return ''

      const parser = new DOMParser()
      const doc = parser.parseFromString(String(html), 'text/html')
      const source = doc.querySelector('table')
      if (!source) return ''

      const table = source.cloneNode(true)

      table.removeAttribute('cellspacing')
      table.removeAttribute('cellpadding')
      table.removeAttribute('width')
      table.removeAttribute('height')

      table.querySelectorAll('script, iframe, object, embed, form').forEach(el => el.remove())

      table.querySelectorAll('*').forEach(el => {
        Array.from(el.attributes).forEach(attr => {
          if (/^on/i.test(attr.name)) {
            el.removeAttribute(attr.name)
          }
        })
      })

      table.querySelectorAll('[contenteditable]').forEach(el => {
        el.removeAttribute('contenteditable')
      })

      return table.outerHTML
    },

    getTableHtml(block) {
      if (!block) {
        return '<table><tbody><tr><td></td></tr></tbody></table>'
      }

      if (/<table\b[\s\S]*?>[\s\S]*?<\/table>/i.test(String(block.text || ''))) {
        const html = this.sanitizeTableHtml(block.text)
        if (html) return html
      }

      if (block.tableData?.rows && Array.isArray(block.tableData.rows)) {
        return this.tableDataToHtml(block.tableData)
      }

      return '<table><tbody><tr><td></td></tr></tbody></table>'
    },

    tableDataToHtml(tableData) {
      const rows = Array.isArray(tableData?.rows) ? tableData.rows : [[]]

      const table = document.createElement('table')
      const tbody = document.createElement('tbody')
      table.appendChild(tbody)

      rows.forEach(row => {
        const tr = document.createElement('tr')

        ;(row || []).forEach(cell => {
          const tag = cell?.tag === 'th' ? 'th' : 'td'
          const td = document.createElement(tag)

          td.textContent = this.getCellText(cell)

          const colspan = this.getCellColspan(cell)
          const rowspan = this.getCellRowspan(cell)

          if (colspan > 1) td.setAttribute('colspan', String(colspan))
          if (rowspan > 1) td.setAttribute('rowspan', String(rowspan))

          tr.appendChild(td)
        })

        tbody.appendChild(tr)
      })

      return table.outerHTML
    },

    getTableGrid(table) {
      const grid = []
      const rows = Array.from(table?.rows || [])

      rows.forEach((tr, rowIndex) => {
        if (!grid[rowIndex]) grid[rowIndex] = []

        let colIndex = 0

        Array.from(tr.cells).forEach(cell => {
          while (grid[rowIndex][colIndex]) colIndex++

          const rowspan = Math.max(1, parseInt(cell.getAttribute('rowspan') || '1', 10))
          const colspan = Math.max(1, parseInt(cell.getAttribute('colspan') || '1', 10))

          for (let r = 0; r < rowspan; r++) {
            if (!grid[rowIndex + r]) grid[rowIndex + r] = []

            for (let c = 0; c < colspan; c++) {
              grid[rowIndex + r][colIndex + c] = cell
            }
          }

          colIndex += colspan
        })
      })

      return grid
    },

    getTableColumnCount(table) {
      const grid = this.getTableGrid(table)
      return Math.max(1, ...grid.map(row => row?.length || 0))
    },

    getCellPosition(table, targetCell) {
      const grid = this.getTableGrid(table)

      for (let r = 0; r < grid.length; r++) {
        const row = grid[r] || []

        for (let c = 0; c < row.length; c++) {
          if (row[c] === targetCell) {
            return { rowIndex: r, colIndex: c }
          }
        }
      }

      return { rowIndex: -1, colIndex: -1 }
    },

    getSelectedCellPosition() {
      const table = this.getEditorTable()
      const cell = table?.querySelector('.editor-selected-cell')

      if (table && cell) {
        return this.getCellPosition(table, cell)
      }

      return { rowIndex: -1, colIndex: -1 }
    },

    getSelectedCell() {
      return this.getEditorTable()?.querySelector('.editor-selected-cell') || null
    },

    createEmptyCell(tagName = 'td') {
      const cell = document.createElement(tagName)
      cell.setAttribute('contenteditable', 'true')
      cell.setAttribute('spellcheck', 'false')
      cell.setAttribute('tabindex', '0')
      return cell
    },

    getCellSpan(cell) {
      return {
        colspan: Math.max(1, parseInt(cell?.getAttribute('colspan') || '1', 10)),
        rowspan: Math.max(1, parseInt(cell?.getAttribute('rowspan') || '1', 10))
      }
    },

    insertCellAtLogicalColumn(table, tr, colIndex, cell) {
      for (const existing of Array.from(tr.cells)) {
        const pos = this.getCellPosition(table, existing)
        const rowIndex = Array.from(table.rows).indexOf(tr)

        if (pos.rowIndex === rowIndex && pos.colIndex > colIndex) {
          tr.insertBefore(cell, existing)
          return
        }
      }

      tr.appendChild(cell)
    },

    applyCellSpan() {
      const cell = this.getSelectedCell()
      if (!cell) return

      const table = this.getEditorTable()
      if (!table) return

      const grid = this.getTableGrid(table)
      const pos = this.getCellPosition(table, cell)
      if (pos.rowIndex < 0 || pos.colIndex < 0) return

      const currentColspan = this.getCellSpan(cell).colspan
      const currentRowspan = this.getCellSpan(cell).rowspan

      let targetCol = Math.max(1, parseInt(this.targetColspan) || 1)
      let targetRow = Math.max(1, parseInt(this.targetRowspan) || 1)

      const maxCol = this.getTableColumnCount(table) - pos.colIndex
      const maxRow = table.rows.length - pos.rowIndex

      targetCol = Math.min(targetCol, maxCol)
      targetRow = Math.min(targetRow, maxRow)

      if (targetCol === currentColspan && targetRow === currentRowspan) return

      const operations = {
        remove: new Set(),
        insert: []
      }

      const filledPositions = new Set()

      for (let r = pos.rowIndex; r < pos.rowIndex + targetRow; r++) {
        for (let c = pos.colIndex; c < pos.colIndex + targetCol; c++) {
          filledPositions.add(`${r},${c}`)
        }
      }

      for (let r = 0; r < grid.length; r++) {
        for (let c = 0; c < (grid[r]?.length || 0); c++) {
          const currentCell = grid[r][c]
          if (!currentCell || operations.remove.has(currentCell)) continue
          if (currentCell === cell) continue

          if (
              r >= pos.rowIndex &&
              r < pos.rowIndex + targetRow &&
              c >= pos.colIndex &&
              c < pos.colIndex + targetCol
          ) {
            if (currentCell.textContent.trim()) {
              cell.innerHTML += (cell.innerHTML ? ' ' : '') + currentCell.innerHTML
            }

            operations.remove.add(currentCell)

            const cRowspan = this.getCellSpan(currentCell).rowspan
            const cColspan = this.getCellSpan(currentCell).colspan

            for (let tr = r; tr < r + cRowspan; tr++) {
              for (let tc = c; tc < c + cColspan; tc++) {
                if (!filledPositions.has(`${tr},${tc}`)) {
                  operations.insert.push({ rowIndex: tr, logicalColIndex: tc })
                  filledPositions.add(`${tr},${tc}`)
                }
              }
            }
          }
        }
      }

      for (let r = pos.rowIndex; r < pos.rowIndex + currentRowspan; r++) {
        for (let c = pos.colIndex; c < pos.colIndex + currentColspan; c++) {
          if (!filledPositions.has(`${r},${c}`)) {
            operations.insert.push({ rowIndex: r, logicalColIndex: c })
            filledPositions.add(`${r},${c}`)
          }
        }
      }

      operations.remove.forEach(c => c.remove())

      const insertByRow = {}
      operations.insert.forEach(op => {
        if (!insertByRow[op.rowIndex]) insertByRow[op.rowIndex] = []
        insertByRow[op.rowIndex].push(op)
      })

      for (const rStr in insertByRow) {
        const r = parseInt(rStr)
        const tr = table.rows[r]
        if (!tr) continue

        const ops = insertByRow[r].sort((a, b) => a.logicalColIndex - b.logicalColIndex)

        for (const op of ops) {
          const newCell = this.createEmptyCell()
          this.insertCellAtLogicalColumn(table, tr, op.logicalColIndex, newCell)
        }
      }

      if (targetCol > 1) {
        cell.setAttribute('colspan', String(targetCol))
      } else {
        cell.removeAttribute('colspan')
      }

      if (targetRow > 1) {
        cell.setAttribute('rowspan', String(targetRow))
      } else {
        cell.removeAttribute('rowspan')
      }

      this.targetColspan = targetCol
      this.targetRowspan = targetRow

      this.syncTableEditorState()

      this.$nextTick(() => {
        this.selectTableCell(cell)
      })
    },

    insertTableRow(position) {
      const table = this.getEditorTable()
      if (!table) return

      const selected = this.getSelectedCell()
      const rowCount = table.rows.length
      const selectedPos = selected
          ? this.getCellPosition(table, selected)
          : { rowIndex: rowCount - 1, colIndex: 0 }

      let targetRowIndex = position === 'above' ? selectedPos.rowIndex : selectedPos.rowIndex + 1
      targetRowIndex = Math.max(0, Math.min(rowCount, targetRowIndex))

      const newRow = table.insertRow(targetRowIndex)
      const columnCount = this.getTableColumnCount(table)
      const grid = this.getTableGrid(table)
      const occupied = grid[targetRowIndex] || []

      for (let col = 0; col < columnCount; col++) {
        if (occupied[col]) continue
        newRow.appendChild(this.createEmptyCell())
      }

      if (!newRow.cells.length) {
        newRow.appendChild(this.createEmptyCell())
      }

      this.clearTableSelection()

      if (newRow.cells[0]) {
        this.selectTableCell(newRow.cells[0])
      }

      this.syncTableEditorState()
    },

    insertTableColumn(position) {
      const table = this.getEditorTable()
      if (!table) return

      const selected = this.getSelectedCell()
      const oldColumnCount = this.getTableColumnCount(table)
      const current = selected
          ? this.getCellPosition(table, selected)
          : { rowIndex: 0, colIndex: oldColumnCount - 1 }

      let targetCol = position === 'left' ? current.colIndex : current.colIndex + 1
      targetCol = Math.max(0, Math.min(oldColumnCount, targetCol))

      const grid = this.getTableGrid(table)
      const expandedCells = new Set()

      Array.from(table.rows).forEach((tr, rowIndex) => {
        const cell = (grid[rowIndex] || [])[targetCol]

        if (cell) {
          const cellPos = this.getCellPosition(table, cell)
          const colspan = Math.max(1, parseInt(cell.getAttribute('colspan') || '1', 10))

          if (targetCol >= cellPos.colIndex && targetCol < cellPos.colIndex + colspan) {
            if (!expandedCells.has(cell)) {
              cell.setAttribute('colspan', String(colspan + 1))
              expandedCells.add(cell)
            }
            return
          }
        }

        const newCell = this.createEmptyCell()
        let logicalCol = 0
        let inserted = false

        for (const existing of Array.from(tr.cells)) {
          const span = Math.max(1, parseInt(existing.getAttribute('colspan') || '1', 10))

          if (logicalCol >= targetCol) {
            tr.insertBefore(newCell, existing)
            inserted = true
            break
          }

          logicalCol += span
        }

        if (!inserted) {
          tr.appendChild(newCell)
        }
      })

      this.clearTableSelection()

      const afterGrid = this.getTableGrid(table)
      const selectedAfter = afterGrid.map(row => row?.[targetCol]).find(cell => !!cell)

      if (selectedAfter) {
        this.selectTableCell(selectedAfter)
        selectedAfter.focus()
      }

      this.syncTableEditorState()
    },

    deleteSelectedTableRow() {
      const table = this.getEditorTable()
      const selected = this.getSelectedCell()

      if (!table || !selected || table.rows.length <= 1) return

      const grid = this.getTableGrid(table)
      const pos = this.getCellPosition(table, selected)
      if (pos.rowIndex < 0) return

      const targetRow = table.rows[pos.rowIndex]
      const targetCells = Array.from(targetRow.cells)

      const fromAbove = new Set(
          (grid[pos.rowIndex] || []).filter(cell => cell && !targetCells.includes(cell))
      )

      fromAbove.forEach(cell => {
        const rowspan = Math.max(1, parseInt(cell.getAttribute('rowspan') || '1', 10))

        if (rowspan > 1) {
          cell.setAttribute('rowspan', String(rowspan - 1))
        } else {
          cell.removeAttribute('rowspan')
        }
      })

      const nextRow = table.rows[pos.rowIndex + 1]

      if (nextRow) {
        targetCells.forEach(cell => {
          const rowspan = Math.max(1, parseInt(cell.getAttribute('rowspan') || '1', 10))
          if (rowspan <= 1) return

          const cellPos = this.getCellPosition(table, cell)
          const clone = cell.cloneNode(true)
          const remaining = rowspan - 1

          if (remaining > 1) {
            clone.setAttribute('rowspan', String(remaining))
          } else {
            clone.removeAttribute('rowspan')
          }

          const nextGrid = this.getTableGrid(table)
          const nextLogicalRow = nextGrid[pos.rowIndex + 1] || []

          let before = null

          for (let c = cellPos.colIndex; c < nextLogicalRow.length; c++) {
            const existing = nextLogicalRow[c]

            if (existing && existing !== cell && existing.parentElement === nextRow) {
              before = existing
              break
            }
          }

          if (before) {
            nextRow.insertBefore(clone, before)
          } else {
            nextRow.appendChild(clone)
          }
        })
      }

      targetRow.remove()
      this.clearTableSelection()

      const fallback = table.rows[Math.min(pos.rowIndex, table.rows.length - 1)]?.cells?.[0]
      if (fallback) {
        this.selectTableCell(fallback)
        fallback.focus()
      }

      this.syncTableEditorState()
    },

    deleteSelectedTableColumn() {
      const table = this.getEditorTable()
      const selected = this.getSelectedCell()

      if (!table || !selected || this.getTableColumnCount(table) <= 1) return

      const pos = this.getCellPosition(table, selected)
      if (pos.colIndex < 0) return

      this.deleteTableColumnAt(table, pos.colIndex)
      this.clearTableSelection()

      const fallback = table.querySelector('td, th')
      if (fallback) {
        this.selectTableCell(fallback)
      }

      this.syncTableEditorState()
    },

    deleteTableColumnAt(table, colIndex) {
      const grid = this.getTableGrid(table)
      const cells = new Set()

      grid.forEach(row => {
        const cell = row?.[colIndex]
        if (cell) cells.add(cell)
      })

      cells.forEach(cell => {
        const colspan = Math.max(1, parseInt(cell.getAttribute('colspan') || '1', 10))
        const tr = cell.closest('tr')

        if (colspan > 1) {
          cell.setAttribute('colspan', String(colspan - 1))
        } else {
          cell.remove()

          if (tr && !tr.cells.length) {
            tr.appendChild(this.createEmptyCell())
          }
        }
      })
    },

    clearTableSelection() {
      const table = this.getEditorTable()
      if (!table) return

      table.querySelectorAll('.editor-selected-cell').forEach(cell => {
        cell.classList.remove('editor-selected-cell')
      })

      this.editActiveCell = {
        rowIndex: -1,
        colIndex: -1
      }
    },

    syncTableEditorState() {
      this.$nextTick(() => {
        this.prepareEditableTable()
      })
    },

    saveTableEdit() {
      const index = this.editingTable.blockIndex
      const block = this.currentPage?.blocks?.[index]

      if (index < 0 || !block) return

      const html = this.serializeEditorTable()
      if (!html) return

      const tableData = this.parseHtmlTable(html)

      this.$emit('update-block', {
        pageIndex: this.currentPageIndex,
        blockIndex: index,
        block: {
          ...block,
          text: html,
          tableData
        }
      })

      this.editingTable = {
        blockIndex: -1,
        originalHtml: ''
      }

      this.editActiveCell = {
        rowIndex: -1,
        colIndex: -1
      }

      this.exitFocus()

      this.$nextTick(() => {
        this.scrollToBlock(index)
      })
    },

    cancelTableEdit() {
      const wasEditing = this.editingTable.blockIndex >= 0
      const editedIndex = this.editingTable.blockIndex

      this.editingTable = {
        blockIndex: -1,
        originalHtml: ''
      }

      this.editActiveCell = {
        rowIndex: -1,
        colIndex: -1
      }

      this.exitFocus()

      if (wasEditing && this.selectedBlockIndex === editedIndex) {
        this.$nextTick(() => {
          this.scrollToBlock(editedIndex)
        })
      }
    },

    isTableBlock(block) {
      return !!block && String(block.label || '').toLowerCase() === 'table'
    },

    parseHtmlTable(html) {
      if (!html) return { rows: [[]] }

      const parser = new DOMParser()
      const doc = parser.parseFromString(String(html), 'text/html')
      const table = doc.querySelector('table')

      if (!table) return { rows: [[]] }

      const rows = []

      table.querySelectorAll('tr').forEach(tr => {
        const cells = []

        Array.from(tr.children)
            .filter(el => ['TD', 'TH'].includes(el.tagName))
            .forEach(cell => {
              cells.push({
                text: cell.textContent.replace(/\r/g, ''),
                colspan: Math.max(1, parseInt(cell.getAttribute('colspan') || '1', 10)),
                rowspan: Math.max(1, parseInt(cell.getAttribute('rowspan') || '1', 10)),
                tag: cell.tagName.toLowerCase()
              })
            })

        if (cells.length) {
          rows.push(cells)
        }
      })

      return {
        rows: rows.length ? rows : [[]]
      }
    },

    getCellText(cell) {
      return cell && typeof cell === 'object' ? (cell.text ?? '') : (cell ?? '')
    },

    getCellColspan(cell) {
      return cell && typeof cell === 'object' && Number(cell.colspan) > 1 ? Number(cell.colspan) : 1
    },

    getCellRowspan(cell) {
      return cell && typeof cell === 'object' && Number(cell.rowspan) > 1 ? Number(cell.rowspan) : 1
    }
  }
}
</script>

<style scoped>
.preview-center {
  --primary: #4f6ef7;
  --primary-hover: #405be5;
  --primary-soft: #f1f4ff;
  --text: #202633;
  --text-secondary: #667085;
  --muted: #98a2b3;
  --border: #e6e9ef;
  --canvas: #f4f6fa;

  flex: 1;
  min-width: 0;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  background: var(--canvas);
  color: var(--text);
}

.center-header {
  height: 52px;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 0 18px;
  background: rgba(255, 255, 255, .98);
  border-bottom: 1px solid var(--border);
}

.source-tag {
  height: 28px;
  padding: 0 10px;
  display: inline-flex;
  align-items: center;
  border-radius: 7px;
  background: var(--primary-soft);
  color: var(--primary);
  font-size: 12px;
  font-weight: 650;
}

.file-meta {
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 7px;
  font-size: 13px;
}

.meta-icon {
  opacity: .75;
}

.meta-name {
  max-width: 420px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: #374151;
  font-weight: 520;
}

.meta-sep {
  color: #d4d8e1;
}

.meta-size {
  color: var(--muted);
}

.focus-indicator {
  margin-left: auto;
  display: flex;
  align-items: center;
  gap: 9px;
  padding: 5px 7px 5px 9px;
  background: #fff9ed;
  border: 1px solid #ffe1a8;
  border-radius: 8px;
  animation: slideIn .18s ease;
}

.focus-tag {
  padding: 3px 7px;
  border-radius: 5px;
  background: #fff0d2;
  color: #d97706;
  font-size: 11px;
  font-weight: 650;
}

.focus-info {
  color: #b45309;
  font-size: 12px;
}

.exit-focus-btn {
  height: 26px;
  padding: 0 9px;
  display: inline-flex;
  align-items: center;
  gap: 4px;
  border: 1px solid #f2b866;
  border-radius: 6px;
  background: #fff;
  color: #c56a00;
  font-size: 12px;
  cursor: pointer;
  transition: .18s ease;
}

.exit-focus-btn:hover {
  background: #f59e0b;
  border-color: #f59e0b;
  color: #fff;
}

.preview-body {
  flex: 1;
  min-height: 0;
  display: flex;
  overflow: hidden;
}

.preview-panel {
  min-width: 0;
  min-height: 0;
  overflow: auto;
}

.image-panel {
  flex: 0 0 42%;
  max-width: 50%;
  position: relative;
  padding: 20px;
  background:
      radial-gradient(circle at 1px 1px, rgba(100, 116, 139, .11) 1px, transparent 0) 0 0 / 18px 18px,
      #f1f3f7;
  border-right: 1px solid var(--border);
  overflow: auto;
}

.image-viewport {
  min-width: 100%;
  min-height: 100%;
  display: flex;
}

.image-wrapper {
  margin: auto;
  position: relative;
  display: inline-block;
  line-height: 0;
  font-size: 0;
  transform-origin: center center;
  transition: transform .18s ease;
  will-change: transform;
}

.preview-image {
  display: block;
  max-width: min(100%, 1400px);
  max-height: calc(100vh - 220px);
  object-fit: contain;
  border-radius: 5px;
  background: #fff;
  box-shadow: 0 10px 30px rgba(15, 23, 42, .13);
  user-select: none;
}

.overlay {
  position: absolute;
  inset: 0;
  pointer-events: none;
}

.ocr-box {
  position: absolute;
  border: 1.5px solid rgba(79, 110, 247, .55);
  background: rgba(79, 110, 247, .055);
  border-radius: 3px;
  pointer-events: auto;
  cursor: pointer;
  transition: opacity .18s, background .18s, border-color .18s, box-shadow .18s;
}

.ocr-box:hover {
  border-color: rgba(79, 110, 247, .95);
  background: rgba(79, 110, 247, .13);
}

.ocr-box.active {
  border-color: #ef4444;
  background: rgba(239, 68, 68, .12);
  box-shadow: 0 0 0 2px rgba(239, 68, 68, .16);
  z-index: 10;
}

.ocr-box.focus-hidden {
  opacity: 0;
  pointer-events: none;
}

.ocr-box.focus-visible {
  border-color: #ef4444;
  background: rgba(239, 68, 68, .18);
  box-shadow: 0 0 0 3px rgba(239, 68, 68, .22);
  z-index: 20;
  animation: pulse 1.7s infinite;
}

.content-panel {
  flex: 1;
  min-width: 0;
  background: #fff;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.blocks-list {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 10px;
  scrollbar-width: thin;
}

.block-item {
  position: relative;
  display: flex;
  align-items: flex-start;
  gap: 10px;
  margin-bottom: 7px;
  padding: 11px 12px;
  border: 1px solid transparent;
  border-radius: 10px;
  cursor: pointer;
  transition: .18s ease;
}

.block-item:hover {
  background: #f8f9fd;
  border-color: #eef0f5;
}

.block-item.active {
  background: #f6f8ff;
  border-color: #dfe5ff;
  box-shadow: 0 2px 8px rgba(79, 110, 247, .06);
}

.block-item.is-dirty {
  background: #fffbef;
  border-color: #ffe3a8;
}

.block-item.focus-hidden {
  display: none;
}

.block-item.focus-visible {
  margin: 5px 2px;
  padding: 14px;
  background: #fff;
  border: 1px solid #cfd7ff;
  border-radius: 12px;
  box-shadow: 0 8px 24px rgba(79, 110, 247, .12);
  animation: fadeIn .22s ease;
}

.block-index {
  width: 24px;
  height: 24px;
  flex: 0 0 24px;
  margin-top: 1px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 7px;
  background: #f0f2f6;
  color: #8b95a7;
  font-size: 11px;
  font-weight: 650;
}

.block-item.active .block-index,
.block-item.is-dirty .block-index {
  background: var(--primary);
  color: #fff;
}

.block-content {
  flex: 1;
  min-width: 0;
}

.table-preview-card {
  overflow: hidden;
  background: #fff;
  border: 1px solid #e5e8ef;
  border-radius: 11px;
  box-shadow: 0 2px 8px rgba(15, 23, 42, .035);
  transition: .18s ease;
}

.table-preview-card:hover {
  border-color: #bfc9ff;
  box-shadow: 0 6px 18px rgba(79, 110, 247, .09);
}

.table-preview-header {
  min-height: 40px;
  padding: 8px 12px;
  display: flex;
  align-items: center;
  gap: 7px;
  background: #f7f8ff;
  border-bottom: 1px solid #e8ebf7;
}

.table-icon {
  font-size: 15px;
}

.table-title {
  color: #4358d9;
  font-size: 12px;
  font-weight: 650;
}

.edit-hint {
  margin-left: auto;
  padding: 3px 8px;
  border: 1px solid #e3e6ed;
  border-radius: 99px;
  background: #fff;
  color: #9aa2b1;
  font-size: 10px;
}

.table-preview-body {
  padding: 12px;
  overflow: auto;
  background: #fff;
}

.raw-table-host {
  width: 100%;
  overflow: auto;
  color: #303846;
}

.raw-table-host :deep(table),
.table-edit-area :deep(table.editable-table) {
  width: max-content;
  min-width: 100%;
  max-width: none;
  border-collapse: collapse !important;
  background: #fff;
  font-size: 13px;
  line-height: 1.6;
  border: none !important;
}

.raw-table-host :deep(td),
.raw-table-host :deep(th),
.table-edit-area :deep(table.editable-table td),
.table-edit-area :deep(table.editable-table th) {
  padding: 8px 10px !important;
  border: 1px solid #cbd5e1 !important;
  vertical-align: top;
  text-align: left;
  white-space: pre-wrap;
  word-break: break-word;
  min-width: 60px;
  background-color: #fff !important;
}

.raw-table-host :deep(th),
.table-edit-area :deep(table.editable-table th) {
  background-color: #f8fafc !important;
  font-weight: 600;
  color: #374151;
}

.table-edit-mode {
  overflow: hidden;
  background: #fff;
  border: 1px solid #cfd5e2;
  border-radius: 11px;
  box-shadow: 0 8px 24px rgba(15, 23, 42, .09);
}

.table-toolbar {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 16px;
  background: linear-gradient(to bottom, #f8f9fc, #f1f3f8);
  border-bottom: 1px solid #e2e6ef;
  flex-wrap: wrap;
}

.toolbar-left,
.toolbar-middle,
.toolbar-right {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.toolbar-middle {
  margin-left: auto;
}

.toolbar-right {
  margin-left: 12px;
}

.tool-group {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 4px 8px;
  background: rgba(255, 255, 255, 0.6);
  border: 1px solid #e8ebf2;
  border-radius: 8px;
}

.group-label {
  color: #8b95a7;
  font-size: 11px;
  font-weight: 600;
  margin-right: 4px;
}

.toolbar-divider {
  width: 1px;
  height: 24px;
  margin: 0 4px;
  background: #e1e5ec;
}

.table-edit-title {
  color: #202633;
  font-size: 13px;
  font-weight: 700;
}

.table-edit-desc {
  color: #98a0af;
  font-size: 11px;
}

.table-edit-tip {
  padding: 8px 12px;
  background: #fafbff;
  border-bottom: 1px solid #edf0f6;
  color: #7a8495;
  font-size: 11px;
  line-height: 1.5;
}

.tb-btn {
  min-height: 30px;
  padding: 4px 10px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 1px solid #d9dde6;
  border-radius: 6px;
  background: #fff;
  color: #586174;
  font-size: 12px;
  font-weight: 550;
  cursor: pointer;
  white-space: nowrap;
  transition: .16s ease;
}

.tb-btn:hover:not(:disabled) {
  color: var(--primary);
  border-color: #bfc9ff;
  background: #f6f8ff;
}

.tb-btn:disabled {
  opacity: .35;
  cursor: not-allowed;
}

.tb-btn.danger {
  color: #e05252;
  border-color: #f2cccc;
}

.tb-btn.danger:hover:not(:disabled) {
  background: #fff1f1;
  border-color: #ef8f8f;
}

.tb-btn.save {
  color: #fff;
  background: var(--primary);
  border-color: var(--primary);
  box-shadow: 0 2px 7px rgba(79, 110, 247, .2);
}

.tb-btn.save:hover {
  background: var(--primary-hover);
}

.tb-btn.primary {
  background: #f97316;
  border-color: #f97316;
  color: #fff;
}

.tb-btn.primary:hover:not(:disabled) {
  background: #ea580c;
  border-color: #ea580c;
  color: #fff;
}

.span-group {
  background: #fff7ed;
  border-color: #fed7aa;
}

.span-inputs {
  display: flex;
  align-items: center;
  gap: 6px;
}

.span-label {
  font-size: 11px;
  color: #c2410c;
  font-weight: 600;
}

.split-input {
  width: 52px;
  height: 26px;
  text-align: center;
  border: 1px solid #fdba74;
  border-radius: 4px;
  background: #fff;
  color: #c2410c;
  font-size: 12px;
  font-weight: 600;
  outline: none;
}

.split-input:focus {
  border-color: #f97316;
  box-shadow: 0 0 0 2px rgba(249, 115, 22, 0.2);
}

.table-edit-area {
  max-height: 55vh;
  padding: 16px;
  overflow: auto;
  background: #f5f7fa;
}

.table-edit-area :deep(table.editable-table td),
.table-edit-area :deep(table.editable-table th) {
  outline: none;
  cursor: text;
  transition: all 0.15s ease;
}

.table-edit-area :deep(table.editable-table td:hover),
.table-edit-area :deep(table.editable-table th:hover) {
  background-color: #f3f4f6 !important;
}

.table-edit-area :deep(table.editable-table td:focus),
.table-edit-area :deep(table.editable-table th:focus) {
  background-color: #eff6ff !important;
  box-shadow: inset 0 0 0 2px #3b82f6;
  z-index: 1;
}

.table-edit-area :deep(table.editable-table .editor-selected-cell) {
  background-color: #dbeafe !important;
  box-shadow: inset 0 0 0 2px #2563eb;
  z-index: 2;
}

.text-card {
  padding: 12px;
  background: #fafbfc;
  border: 1px solid #e7e9ee;
  border-radius: 10px;
  transition: .18s ease;
}

.text-card:hover {
  border-color: #d3d8e3;
  box-shadow: 0 3px 10px rgba(15, 23, 42, .045);
}

.block-item.is-dirty .text-card {
  background: #fff;
  border-color: #ffd88a;
}

.text-header {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 6px;
}

.text-icon {
  font-size: 13px;
}

.text-title {
  color: #687284;
  font-size: 11px;
  font-weight: 600;
}

.unsaved-badge {
  margin-left: auto;
  padding: 2px 7px;
  border: 1px solid #ffd591;
  border-radius: 99px;
  background: #fff7e6;
  color: #d97706;
  font-size: 10px;
}

.block-text-input {
  width: 100%;
  min-height: 58px;
  padding: 7px 8px;
  border: 1px solid transparent;
  border-radius: 7px;
  outline: none;
  resize: none;
  box-sizing: border-box;
  background: transparent;
  color: #303846;
  font: inherit;
  font-size: 13px;
  line-height: 1.65;
  cursor: default;
  transition: .16s ease;
}

.block-text-input:hover {
  background: #f3f5f8;
}

.block-text-input.is-editing {
  background: #fff;
  border-color: var(--primary);
  resize: vertical;
  cursor: text;
  box-shadow: 0 0 0 3px rgba(79, 110, 247, .08);
}

.block-text-input.is-editing:focus {
  box-shadow: 0 0 0 3px rgba(79, 110, 247, .12);
}

.edit-actions {
  display: flex;
  justify-content: flex-end;
  gap: 7px;
  margin-top: 9px;
  padding-top: 9px;
  border-top: 1px dashed #e2e5eb;
}

.save-btn,
.cancel-btn {
  min-height: 29px;
  padding: 4px 11px;
  display: inline-flex;
  align-items: center;
  gap: 4px;
  border-radius: 6px;
  font-size: 11px;
  font-weight: 600;
  cursor: pointer;
  transition: .16s ease;
}

.save-btn {
  color: #fff;
  background: var(--primary);
  border: 1px solid var(--primary);
}

.save-btn:hover {
  background: var(--primary-hover);
}

.cancel-btn {
  color: #687284;
  background: #fff;
  border: 1px solid #d9dde5;
}

.cancel-btn:hover {
  color: #e05252;
  border-color: #efaaaa;
}

.block-meta {
  margin-top: 6px;
  text-align: right;
  color: #a3aab8;
  font-size: 10px;
}

.empty-state,
.error-state,
.parsing-state {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 10px;
  padding: 40px;
}

.empty-state {
  color: #aab1be;
}

.empty-icon {
  font-size: 34px;
  opacity: .55;
}

.empty-text {
  font-size: 12px;
}

.error-state {
  color: #dc4c4c;
}

.error-icon {
  font-size: 38px;
}

.error-text {
  max-width: 80%;
  text-align: center;
  font-size: 13px;
  line-height: 1.6;
}

.preview-toolbar {
  height: 54px;
  flex-shrink: 0;
  padding: 0 14px;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 7px;
  background: rgba(255, 255, 255, .98);
  border-top: 1px solid var(--border);
  box-shadow: 0 -3px 12px rgba(15, 23, 42, .035);
}

.tool-btn {
  width: 31px;
  height: 31px;
  display: flex;
  align-items: center;
  justify-content: center;
  border: 1px solid #d9dde5;
  border-radius: 7px;
  background: #fff;
  color: #5d6676;
  font-size: 14px;
  cursor: pointer;
  transition: .16s ease;
}

.tool-btn:hover:not(:disabled) {
  color: var(--primary);
  border-color: #bfc9ff;
  background: #f7f8ff;
}

.tool-btn:disabled {
  opacity: .35;
  cursor: not-allowed;
}

.tool-btn.export-btn {
  width: auto;
  padding: 0 14px;
  gap: 6px;
  font-size: 12px;
  font-weight: 600;
  color: var(--primary);
  border-color: #c7d2fe;
  background: #eef2ff;
}

.tool-btn.export-btn:hover {
  background: var(--primary);
  color: #fff;
  border-color: var(--primary);
}

.zoom-value {
  min-width: 48px;
  text-align: center;
  color: #687284;
  font-size: 11px;
  font-variant-numeric: tabular-nums;
}

.toolbar-sep {
  width: 1px;
  height: 20px;
  margin: 0 4px;
  background: #e4e7ed;
}

.page-input {
  height: 31px;
  padding: 0 8px;
  display: flex;
  align-items: center;
  gap: 5px;
  border: 1px solid #d9dde5;
  border-radius: 7px;
  background: #fff;
  color: #788293;
  font-size: 12px;
}

.page-input:focus-within {
  border-color: #bfc9ff;
  box-shadow: 0 0 0 3px rgba(79, 110, 247, .06);
}

.page-input input {
  width: 32px;
  padding: 0;
  border: 0;
  outline: 0;
  background: transparent;
  color: #303846;
  text-align: center;
  font: inherit;
}

.parsing-state {
  flex: 1;
}

.parsing-dots {
  display: flex;
  align-items: center;
  gap: 7px;
}

.parsing-dots .dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #6378f7;
  animation: dotFlashing 1s infinite ease-in-out alternate;
}

.parsing-dots .dot:nth-child(2) {
  animation-delay: .18s;
}

.parsing-dots .dot:nth-child(3) {
  animation-delay: .36s;
}

.parsing-text {
  color: #8b95a7;
  font-size: 13px;
}

.export-wrapper {
  position: relative;
  display: inline-flex;
}

.export-arrow {
  font-size: 10px;
  margin-left: 4px;
  transition: transform 0.2s ease;
  opacity: 0.7;
}

.export-dropdown {
  position: absolute;
  bottom: calc(100% + 8px);
  right: 0;
  min-width: 220px;
  background: #fff;
  border: 1px solid #e2e6ef;
  border-radius: 8px;
  box-shadow: 0 6px 16px rgba(15, 23, 42, .1);
  z-index: 100;
  padding: 8px;
  animation: fadeIn 0.15s ease;
}

.dropdown-header {
  padding: 4px 8px 8px;
  font-size: 12px;
  color: #667085;
  font-weight: 600;
  border-bottom: 1px solid #f0f2f6;
  margin-bottom: 4px;
}

.dropdown-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 8px;
  font-size: 12px;
  color: #374151;
  border-radius: 6px;
  cursor: pointer;
  transition: background 0.15s ease;
}

.dropdown-item:hover {
  background: #f3f4f6;
}

.dropdown-item input[type="checkbox"] {
  accent-color: var(--primary);
  width: 14px;
  height: 14px;
  cursor: pointer;
}

.dropdown-footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 8px;
  padding-top: 8px;
  border-top: 1px solid #f0f2f6;
}

@keyframes dotFlashing {
  0% {
    opacity: .35;
    transform: translateY(1px);
  }
  100% {
    opacity: 1;
    transform: translateY(-2px);
  }
}

@keyframes pulse {
  0%,
  100% {
    box-shadow: 0 0 0 3px rgba(239, 68, 68, .22);
  }
  50% {
    box-shadow: 0 0 0 7px rgba(239, 68, 68, .08);
  }
}

@keyframes slideIn {
  from {
    opacity: 0;
    transform: translateX(8px);
  }
  to {
    opacity: 1;
    transform: translateX(0);
  }
}

@keyframes fadeIn {
  from {
    opacity: 0;
    transform: translateY(7px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

@media (max-width: 1050px) {
  .image-panel {
    flex: 0 0 36%;
    max-width: 42%;
    padding: 14px;
  }

  .content-panel {
    flex: 1;
  }

  .meta-name {
    max-width: 260px;
  }

  .table-toolbar {
    align-items: flex-start;
  }

  .toolbar-middle {
    margin-left: 0;
    width: 100%;
  }

  .toolbar-divider {
    display: none;
  }
}

@media (max-width: 760px) {
  .preview-body {
    flex-direction: column;
  }

  .image-panel {
    flex: 0 0 32vh;
    min-height: 220px;
    max-width: 100%;
    border-right: 0;
    border-bottom: 1px solid var(--border);
  }

  .content-panel {
    flex: 1;
  }

  .center-header {
    padding: 0 10px;
  }

  .focus-info,
  .edit-hint {
    display: none;
  }
}
</style>