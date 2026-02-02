/**
 * SceneSettingsPage Component
 * Settings and customization for scene organizers
 */

import React, { useState, useEffect } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { useEntityStore } from '../stores/entityStore';
import { useToastStore } from '../stores/toastStore';
import { Scene, Palette } from '../types/scene';
import { useTranslation } from 'react-i18next';

export const SceneSettingsPage: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { t } = useTranslation();
  const fetchScene = useEntityStore((state) => state.fetchScene);
  const updateScene = useEntityStore((state) => state.updateScene);
  const addToast = useToastStore((state) => state.addToast);

  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [scene, setScene] = useState<Scene | null>(null);
  
  // Form state
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [tags, setTags] = useState<string[]>([]);
  const [tagInput, setTagInput] = useState('');
  const [visibility, setVisibility] = useState<'public' | 'private' | 'unlisted'>('public');
  const [palette, setPalette] = useState<Palette>({
    primary: '#3b82f6',
    secondary: '#8b5cf6',
    accent: '#ec4899',
    background: '#ffffff',
    text: '#000000',
  });

  // Load scene data
  useEffect(() => {
    if (!id) return;

    const loadScene = async () => {
      try {
        setLoading(true);
        const sceneData = await fetchScene(id);
        setScene(sceneData);
        
        // Populate form
        setName(sceneData.name || '');
        setDescription(sceneData.description || '');
        setTags(sceneData.tags || []);
        setVisibility(sceneData.visibility as 'public' | 'private' | 'unlisted' || 'public');
        if (sceneData.palette) {
          setPalette(sceneData.palette);
        }
      } catch (error) {
        addToast({
          type: 'error',
          message: t('errors.failedToLoadScene', 'Failed to load scene'),
        });
        navigate('/');
      } finally {
        setLoading(false);
      }
    };

    loadScene();
  }, [id, fetchScene, navigate, addToast, t]);

  const handleSave = async () => {
    if (!id || !scene) return;

    try {
      setSaving(true);
      await updateScene(id, {
        name: name.trim(),
        description: description.trim(),
        tags,
        visibility,
        palette,
      });
      addToast({
        type: 'success',
        message: t('scene.settings.saved', 'Scene settings saved successfully'),
      });
    } catch (error) {
      addToast({
        type: 'error',
        message: t('errors.failedToSaveScene', 'Failed to save scene settings'),
      });
    } finally {
      setSaving(false);
    }
  };

  const handleAddTag = () => {
    const trimmed = tagInput.trim();
    if (trimmed && !tags.includes(trimmed)) {
      setTags([...tags, trimmed]);
      setTagInput('');
    }
  };

  const handleRemoveTag = (tagToRemove: string) => {
    setTags(tags.filter((tag) => tag !== tagToRemove));
  };

  const handleKeyPress = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
      e.preventDefault();
      handleAddTag();
    }
  };

  if (loading) {
    return (
      <div className="min-h-screen bg-background text-foreground p-8">
        <div className="max-w-4xl mx-auto">
          <div className="animate-pulse">
            <div className="h-8 bg-background-secondary rounded w-1/3 mb-8"></div>
            <div className="space-y-6">
              <div className="h-32 bg-background-secondary rounded"></div>
              <div className="h-32 bg-background-secondary rounded"></div>
              <div className="h-32 bg-background-secondary rounded"></div>
            </div>
          </div>
        </div>
      </div>
    );
  }

  if (!scene) {
    return (
      <div className="min-h-screen bg-background text-foreground p-8">
        <div className="max-w-4xl mx-auto">
          <p className="text-foreground-secondary">{t('scene.notFound', 'Scene not found')}</p>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-background text-foreground p-8">
      <div className="max-w-4xl mx-auto">
        {/* Header */}
        <div className="mb-8">
          <h1 className="text-4xl font-bold mb-2 text-foreground">
            {t('scene.settings.title', 'Scene Settings')}
          </h1>
          <p className="text-foreground-secondary">
            {t('scene.settings.subtitle', 'Customize your scene appearance and settings')}
          </p>
        </div>

        <div className="space-y-6">
          {/* Basic Information */}
          <section className="bg-background-secondary border border-border rounded-lg p-6">
            <h2 className="text-2xl font-semibold mb-4 text-foreground">
              {t('scene.settings.basicInfo', 'Basic Information')}
            </h2>
            
            <div className="space-y-4">
              {/* Scene Name */}
              <div>
                <label className="block text-sm font-medium text-foreground mb-2">
                  {t('scene.name', 'Scene Name')}
                </label>
                <input
                  type="text"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  className="w-full px-4 py-2 bg-background border border-border rounded-lg text-foreground focus:outline-none focus:ring-2 focus:ring-brand-primary"
                  placeholder={t('scene.namePlaceholder', 'Enter scene name')}
                  maxLength={64}
                  minLength={3}
                />
              </div>

              {/* Description */}
              <div>
                <label className="block text-sm font-medium text-foreground mb-2">
                  {t('scene.description', 'Description')}
                </label>
                <textarea
                  value={description}
                  onChange={(e) => setDescription(e.target.value)}
                  className="w-full px-4 py-2 bg-background border border-border rounded-lg text-foreground focus:outline-none focus:ring-2 focus:ring-brand-primary"
                  placeholder={t('scene.descriptionPlaceholder', 'Enter scene description')}
                  rows={4}
                />
              </div>
            </div>
          </section>

          {/* Tags/Genres */}
          <section className="bg-background-secondary border border-border rounded-lg p-6">
            <h2 className="text-2xl font-semibold mb-4 text-foreground">
              {t('scene.settings.tags', 'Tags & Genres')}
            </h2>
            
            <div className="space-y-4">
              {/* Tag Input */}
              <div>
                <label className="block text-sm font-medium text-foreground mb-2">
                  {t('scene.addTag', 'Add Tag')}
                </label>
                <div className="flex gap-2">
                  <input
                    type="text"
                    value={tagInput}
                    onChange={(e) => setTagInput(e.target.value)}
                    onKeyPress={handleKeyPress}
                    className="flex-1 px-4 py-2 bg-background border border-border rounded-lg text-foreground focus:outline-none focus:ring-2 focus:ring-brand-primary"
                    placeholder={t('scene.tagPlaceholder', 'e.g., techno, underground, experimental')}
                  />
                  <button
                    onClick={handleAddTag}
                    className="px-6 py-2 bg-brand-primary hover:bg-brand-primary-dark text-white rounded-lg transition-colors"
                  >
                    {t('common.add', 'Add')}
                  </button>
                </div>
              </div>

              {/* Tag List */}
              {tags.length > 0 && (
                <div className="flex flex-wrap gap-2">
                  {tags.map((tag) => (
                    <span
                      key={tag}
                      className="inline-flex items-center gap-2 px-3 py-1 bg-brand-underground text-white rounded-full text-sm"
                    >
                      {tag}
                      <button
                        onClick={() => handleRemoveTag(tag)}
                        className="hover:text-red-400 transition-colors"
                        aria-label={t('common.remove', 'Remove')}
                      >
                        ×
                      </button>
                    </span>
                  ))}
                </div>
              )}
            </div>
          </section>

          {/* Privacy Settings */}
          <section className="bg-background-secondary border border-border rounded-lg p-6">
            <h2 className="text-2xl font-semibold mb-4 text-foreground">
              {t('scene.settings.privacy', 'Privacy Settings')}
            </h2>
            
            <div className="space-y-3">
              <label className="flex items-center gap-3 cursor-pointer">
                <input
                  type="radio"
                  name="visibility"
                  value="public"
                  checked={visibility === 'public'}
                  onChange={(e) => setVisibility(e.target.value as 'public')}
                  className="w-4 h-4 text-brand-primary"
                />
                <div>
                  <div className="font-medium text-foreground">
                    {t('scene.visibility.public', 'Public')}
                  </div>
                  <div className="text-sm text-foreground-secondary">
                    {t('scene.visibility.publicDesc', 'Visible to everyone and appears in search')}
                  </div>
                </div>
              </label>

              <label className="flex items-center gap-3 cursor-pointer">
                <input
                  type="radio"
                  name="visibility"
                  value="private"
                  checked={visibility === 'private'}
                  onChange={(e) => setVisibility(e.target.value as 'private')}
                  className="w-4 h-4 text-brand-primary"
                />
                <div>
                  <div className="font-medium text-foreground">
                    {t('scene.visibility.private', 'Private')}
                  </div>
                  <div className="text-sm text-foreground-secondary">
                    {t('scene.visibility.privateDesc', 'Visible only to members')}
                  </div>
                </div>
              </label>

              <label className="flex items-center gap-3 cursor-pointer">
                <input
                  type="radio"
                  name="visibility"
                  value="unlisted"
                  checked={visibility === 'unlisted'}
                  onChange={(e) => setVisibility(e.target.value as 'unlisted')}
                  className="w-4 h-4 text-brand-primary"
                />
                <div>
                  <div className="font-medium text-foreground">
                    {t('scene.visibility.unlisted', 'Unlisted')}
                  </div>
                  <div className="text-sm text-foreground-secondary">
                    {t('scene.visibility.unlistedDesc', 'Hidden from search, accessible with link')}
                  </div>
                </div>
              </label>
            </div>
          </section>

          {/* Color Customization */}
          <section className="bg-background-secondary border border-border rounded-lg p-6">
            <h2 className="text-2xl font-semibold mb-4 text-foreground">
              {t('scene.settings.customization', 'Visual Customization')}
            </h2>
            
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div>
                <label className="block text-sm font-medium text-foreground mb-2">
                  {t('scene.palette.primary', 'Primary Color')}
                </label>
                <div className="flex items-center gap-2">
                  <input
                    type="color"
                    value={palette.primary}
                    onChange={(e) => setPalette({ ...palette, primary: e.target.value })}
                    className="w-12 h-12 rounded border border-border cursor-pointer"
                  />
                  <input
                    type="text"
                    value={palette.primary}
                    onChange={(e) => setPalette({ ...palette, primary: e.target.value })}
                    className="flex-1 px-4 py-2 bg-background border border-border rounded-lg text-foreground focus:outline-none focus:ring-2 focus:ring-brand-primary"
                    placeholder="#3b82f6"
                  />
                </div>
              </div>

              <div>
                <label className="block text-sm font-medium text-foreground mb-2">
                  {t('scene.palette.secondary', 'Secondary Color')}
                </label>
                <div className="flex items-center gap-2">
                  <input
                    type="color"
                    value={palette.secondary}
                    onChange={(e) => setPalette({ ...palette, secondary: e.target.value })}
                    className="w-12 h-12 rounded border border-border cursor-pointer"
                  />
                  <input
                    type="text"
                    value={palette.secondary}
                    onChange={(e) => setPalette({ ...palette, secondary: e.target.value })}
                    className="flex-1 px-4 py-2 bg-background border border-border rounded-lg text-foreground focus:outline-none focus:ring-2 focus:ring-brand-primary"
                    placeholder="#8b5cf6"
                  />
                </div>
              </div>

              <div>
                <label className="block text-sm font-medium text-foreground mb-2">
                  {t('scene.palette.accent', 'Accent Color')}
                </label>
                <div className="flex items-center gap-2">
                  <input
                    type="color"
                    value={palette.accent}
                    onChange={(e) => setPalette({ ...palette, accent: e.target.value })}
                    className="w-12 h-12 rounded border border-border cursor-pointer"
                  />
                  <input
                    type="text"
                    value={palette.accent}
                    onChange={(e) => setPalette({ ...palette, accent: e.target.value })}
                    className="flex-1 px-4 py-2 bg-background border border-border rounded-lg text-foreground focus:outline-none focus:ring-2 focus:ring-brand-primary"
                    placeholder="#ec4899"
                  />
                </div>
              </div>

              <div>
                <label className="block text-sm font-medium text-foreground mb-2">
                  {t('scene.palette.background', 'Background Color')}
                </label>
                <div className="flex items-center gap-2">
                  <input
                    type="color"
                    value={palette.background}
                    onChange={(e) => setPalette({ ...palette, background: e.target.value })}
                    className="w-12 h-12 rounded border border-border cursor-pointer"
                  />
                  <input
                    type="text"
                    value={palette.background}
                    onChange={(e) => setPalette({ ...palette, background: e.target.value })}
                    className="flex-1 px-4 py-2 bg-background border border-border rounded-lg text-foreground focus:outline-none focus:ring-2 focus:ring-brand-primary"
                    placeholder="#ffffff"
                  />
                </div>
              </div>

              <div>
                <label className="block text-sm font-medium text-foreground mb-2">
                  {t('scene.palette.text', 'Text Color')}
                </label>
                <div className="flex items-center gap-2">
                  <input
                    type="color"
                    value={palette.text}
                    onChange={(e) => setPalette({ ...palette, text: e.target.value })}
                    className="w-12 h-12 rounded border border-border cursor-pointer"
                  />
                  <input
                    type="text"
                    value={palette.text}
                    onChange={(e) => setPalette({ ...palette, text: e.target.value })}
                    className="flex-1 px-4 py-2 bg-background border border-border rounded-lg text-foreground focus:outline-none focus:ring-2 focus:ring-brand-primary"
                    placeholder="#000000"
                  />
                </div>
              </div>
            </div>

            {/* Color Preview */}
            <div className="mt-6">
              <h3 className="text-lg font-medium text-foreground mb-3">
                {t('scene.palette.preview', 'Preview')}
              </h3>
              <div
                className="rounded-lg p-6 border"
                style={{
                  backgroundColor: palette.background,
                  color: palette.text,
                  borderColor: palette.primary,
                }}
              >
                <h4 className="text-xl font-bold mb-2" style={{ color: palette.primary }}>
                  {name || t('scene.previewTitle', 'Scene Title')}
                </h4>
                <p className="mb-4">
                  {description || t('scene.previewDescription', 'Scene description will appear here')}
                </p>
                <div className="flex gap-2">
                  <button
                    className="px-4 py-2 rounded"
                    style={{ backgroundColor: palette.primary, color: '#ffffff' }}
                  >
                    {t('common.primary', 'Primary')}
                  </button>
                  <button
                    className="px-4 py-2 rounded"
                    style={{ backgroundColor: palette.secondary, color: '#ffffff' }}
                  >
                    {t('common.secondary', 'Secondary')}
                  </button>
                  <button
                    className="px-4 py-2 rounded"
                    style={{ backgroundColor: palette.accent, color: '#ffffff' }}
                  >
                    {t('common.accent', 'Accent')}
                  </button>
                </div>
              </div>
            </div>
          </section>

          {/* Members & Alliance Section */}
          <section className="bg-background-secondary border border-border rounded-lg p-6">
            <h2 className="text-2xl font-semibold mb-4 text-foreground">
              {t('scene.settings.members', 'Members & Alliances')}
            </h2>
            
            <div className="space-y-4">
              <div className="flex items-center justify-between p-4 bg-background rounded-lg border border-border">
                <div>
                  <div className="font-medium text-foreground">
                    {t('scene.members.manage', 'Member Management')}
                  </div>
                  <div className="text-sm text-foreground-secondary">
                    {t('scene.members.description', 'View and manage scene members and roles')}
                  </div>
                </div>
                <button
                  onClick={() => navigate(`/scenes/${id}/members`)}
                  className="px-4 py-2 bg-brand-primary hover:bg-brand-primary-dark text-white rounded-lg transition-colors"
                >
                  {t('scene.members.view', 'View Members')}
                </button>
              </div>

              <div className="flex items-center justify-between p-4 bg-background rounded-lg border border-border">
                <div>
                  <div className="font-medium text-foreground">
                    {t('scene.alliances.manage', 'Alliance Management')}
                  </div>
                  <div className="text-sm text-foreground-secondary">
                    {t('scene.alliances.description', 'Manage trust relationships with other scenes')}
                  </div>
                </div>
                <button
                  onClick={() => navigate(`/scenes/${id}/alliances`)}
                  className="px-4 py-2 bg-brand-primary hover:bg-brand-primary-dark text-white rounded-lg transition-colors"
                >
                  {t('scene.alliances.view', 'View Alliances')}
                </button>
              </div>
            </div>
          </section>

          {/* Verification Status */}
          <section className="bg-background-secondary border border-border rounded-lg p-6">
            <h2 className="text-2xl font-semibold mb-4 text-foreground">
              {t('scene.settings.verification', 'Verification Status')}
            </h2>
            
            <div className="space-y-4">
              <div className="flex items-center gap-3">
                <div className="w-12 h-12 rounded-full bg-green-100 dark:bg-green-900 flex items-center justify-center">
                  <svg className="w-6 h-6 text-green-600 dark:text-green-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
                  </svg>
                </div>
                <div className="flex-1">
                  <div className="font-medium text-foreground">
                    {t('scene.verification.ownerVerified', 'Owner Verified')}
                  </div>
                  <div className="text-sm text-foreground-secondary">
                    {t('scene.verification.ownerVerifiedDesc', 'Scene owner identity is verified via AT Protocol DID')}
                  </div>
                </div>
              </div>

              <div className="flex items-center gap-3 opacity-60">
                <div className="w-12 h-12 rounded-full bg-gray-100 dark:bg-gray-800 flex items-center justify-center">
                  <svg className="w-6 h-6 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
                  </svg>
                </div>
                <div className="flex-1">
                  <div className="font-medium text-foreground">
                    {t('scene.verification.communityBadge', 'Community Badge')}
                  </div>
                  <div className="text-sm text-foreground-secondary">
                    {t('scene.verification.communityBadgeDesc', 'Coming soon - earn badges through community engagement')}
                  </div>
                </div>
              </div>
            </div>
          </section>

          {/* Action Buttons */}
          <div className="flex justify-end gap-4">
            <button
              onClick={() => navigate(`/scenes/${id}`)}
              className="px-6 py-3 bg-background-secondary hover:bg-underground-lighter border border-border text-foreground rounded-lg transition-colors"
            >
              {t('common.cancel', 'Cancel')}
            </button>
            <button
              onClick={handleSave}
              disabled={saving}
              className="px-6 py-3 bg-brand-primary hover:bg-brand-primary-dark text-white rounded-lg transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {saving ? t('common.saving', 'Saving...') : t('common.save', 'Save Changes')}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
};
