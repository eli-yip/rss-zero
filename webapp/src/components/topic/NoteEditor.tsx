import {
  Button,
  Drawer,
  DrawerBody,
  DrawerContent,
  DrawerFooter,
  DrawerHeader,
  Textarea,
} from "@heroui/react";
import { useCallback, useEffect, useState } from "react";
import { FaCheck, FaEdit, FaExpand, FaEye, FaTimes } from "react-icons/fa";

import { Markdown } from "@/components/topic/Markdown";
import type { BookmarkDataChangeHandler } from "@/components/topic/Topics";
import type { Topic } from "@/types/Topic";

interface NoteEditorProps {
  topic: Topic;
  bookmarkId: string;
  onBookmarkDataChange: BookmarkDataChangeHandler;
}

export function NoteEditor({
  topic,
  bookmarkId,
  onBookmarkDataChange,
}: NoteEditorProps) {
  // 控制抽屉显示
  const [isDrawerOpen, setIsDrawerOpen] = useState(false);
  // 是否处于预览模式
  const [isPreviewMode, setIsPreviewMode] = useState(true);
  // 笔记内容
  const [noteText, setNoteText] = useState(topic.custom?.note || "");
  // 在小屏幕上控制是否显示主题内容
  const [showTopicContent, setShowTopicContent] = useState(false);

  // 处理打开/关闭抽屉
  const handleDrawerToggle = useCallback(() => {
    setIsDrawerOpen((prev) => !prev);
  }, []);

  // 切换预览模式
  const handleTogglePreview = useCallback(() => {
    setIsPreviewMode((prev) => !prev);
  }, []);

  // 切换显示主题内容
  const handleToggleTopicContent = useCallback(() => {
    setShowTopicContent((prev) => !prev);
  }, []);

  // 保存笔记
  const handleSaveNote = useCallback(() => {
    onBookmarkDataChange(topic.id, bookmarkId, { note: noteText }, "note");
    setIsPreviewMode(true);
  }, [topic.id, bookmarkId, noteText, onBookmarkDataChange]);

  // 取消编辑
  const handleCancelEdit = useCallback(() => {
    setNoteText(topic.custom?.note || "");
    setIsPreviewMode(true);
  }, [topic.custom?.note]);

  // 当抽屉关闭时重置状态
  useEffect(() => {
    if (!isDrawerOpen) {
      setIsPreviewMode(true);
      setShowTopicContent(false);
      setNoteText(topic.custom?.note || "");
    }
  }, [isDrawerOpen, topic.custom?.note]);

  return (
    <>
      {/* 编辑笔记按钮 */}
      <Button
        size="sm"
        disableAnimation
        startContent={<FaEdit />}
        onPress={handleDrawerToggle}
      >
        笔记
      </Button>

      {/* 笔记编辑 */}
      <Drawer
        isOpen={isDrawerOpen}
        hideCloseButton
        onClose={handleDrawerToggle}
        placement="bottom"
        size="full"
      >
        <DrawerContent>
          <DrawerHeader className="flex items-center justify-between">
            <div>笔记</div>
            <div className="flex gap-2">
              <Button size="sm" isIconOnly onPress={handleTogglePreview}>
                {isPreviewMode ? <FaEdit /> : <FaEye />}
              </Button>
            </div>
          </DrawerHeader>

          <DrawerBody>
            <div className="flex h-full flex-col gap-4 sm:flex-row">
              <div
                className={`relative flex-1 overflow-auto rounded-2xl border p-4 sm:block ${showTopicContent ? "z-20 block bg-white pt-0 backdrop-blur dark:bg-black" : "hidden"}`}
              >
                <div className="mb-4 items-center">
                  {!showTopicContent && (
                    <h3 className="font-bold text-lg">原文</h3>
                  )}
                </div>
                <Markdown content={topic.body} />
              </div>

              {/* 笔记内容 - 在预览模式或编辑模式之间切换 */}
              <div
                className={`h-full flex-1 ${!showTopicContent || window.innerWidth >= 640 ? "block" : "hidden"}`}
              >
                {/* 预览模式下显示 Markdown 渲染后的内容 */}
                {isPreviewMode ? (
                  <div className="h-full overflow-auto rounded-2xl border p-4">
                    <h3 className="font-bold text-lg">笔记</h3>
                    <div>
                      {noteText ? (
                        <Markdown content={noteText} />
                      ) : (
                        <p className="text-gray-400">暂无笔记内容</p>
                      )}
                    </div>
                  </div>
                ) : (
                  <div className="h-full rounded-2xl border p-4">
                    <Textarea
                      value={noteText}
                      onValueChange={setNoteText}
                      placeholder="在此输入笔记内容，支持 Markdown 格式..."
                      disableAutosize
                      classNames={{
                        base: "!h-full",
                        inputWrapper: "!h-full",
                        input: "!h-full",
                      }}
                    />
                  </div>
                )}
              </div>
            </div>
          </DrawerBody>

          <DrawerFooter>
            <div className="flex justify-end gap-2">
              <Button
                isIconOnly
                className="sm:hidden"
                onPress={handleToggleTopicContent}
              >
                <FaExpand />
              </Button>
              {!isPreviewMode && (
                <>
                  <Button color="danger" isIconOnly onPress={handleCancelEdit}>
                    <FaTimes />
                  </Button>
                  <Button color="primary" isIconOnly onPress={handleSaveNote}>
                    <FaCheck />
                  </Button>
                </>
              )}
              <Button onPress={handleDrawerToggle}>关闭</Button>
            </div>
          </DrawerFooter>
        </DrawerContent>
      </Drawer>
    </>
  );
}
