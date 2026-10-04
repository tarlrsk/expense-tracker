import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'
import { ArrowDownIcon, ArrowUpIcon, ChevronRightIcon } from 'lucide-react'
import { useState } from 'react'
import type { SubmitEvent } from 'react'

import { createCategory, reorderCategories, updateCategory } from '@/api/categories'
import type { Category, CategoryKind, UpdateCategoryRequest } from '@/api/types'
import { CategoryIcon } from '@/components/category-icon'
import { ChoiceGroup } from '@/components/choice-group'
import { BackLink, PageTitle } from '@/components/layout'
import { FormAlert, Notice } from '@/components/messages'
import { TextField } from '@/components/text-field'
import { Button } from '@/components/ui/button'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Skeleton } from '@/components/ui/skeleton'
import { checkCategoryIcon, checkCategoryName, maxCategories, moveItem } from '@/lib/category-form'
import { errorText, isApiError } from '@/lib/errors'
import { categories as t } from '@/messages/categories'
import { common } from '@/messages/common'
import { settings } from '@/messages/settings'
import { categoriesQuery } from '@/queries/money'

// The user's categories (ADR-0061, ADR-0069, ADR-0074): Expenses, Income and Archived. Add,
// rename, change the icon, archive and unarchive open sheets; "Edit order" shows up and down
// buttons and sends the whole active list at once.
export const Route = createFileRoute('/_app/settings/categories')({
  component: CategoriesPage,
})

interface OrderDraft {
  expense: string[]
  income: string[]
}

function activeIds(all: readonly Category[], kind: CategoryKind): string[] {
  return all.filter((c) => c.kind === kind && !c.archived).map((c) => c.id)
}

function CategoriesPage() {
  const queryClient = useQueryClient()
  const categories = useQuery(categoriesQuery)
  const [notice, setNotice] = useState<string | null>(null)
  const [order, setOrder] = useState<OrderDraft | null>(null)
  const [addOpen, setAddOpen] = useState(false)
  const [editTarget, setEditTarget] = useState<Category | null>(null)
  const [editOpen, setEditOpen] = useState(false)
  const [sheetKey, setSheetKey] = useState(0)

  const reorder = useMutation({
    mutationFn: reorderCategories,
    onSuccess: (list) => {
      queryClient.setQueryData(categoriesQuery.queryKey, list)
      setOrder(null)
      setNotice(t.orderSaved)
    },
    onError: (err) => {
      if (isApiError(err, 'conflict')) {
        // The list changed elsewhere (ADR-0069): reload it and start again.
        setOrder(null)
        void queryClient.invalidateQueries({ queryKey: categoriesQuery.queryKey })
      }
    },
  })

  function done(text: string | null) {
    setAddOpen(false)
    setEditOpen(false)
    setNotice(text)
    void queryClient.invalidateQueries({ queryKey: categoriesQuery.queryKey })
  }

  const all = categories.data?.categories ?? []
  const byId = new Map(all.map((c) => [c.id, c]))
  const editing = order !== null

  function startEditing() {
    setNotice(null)
    reorder.reset()
    setOrder({ expense: activeIds(all, 'expense'), income: activeIds(all, 'income') })
  }

  function saveOrder() {
    if (!order || reorder.isPending) {
      return
    }
    const ids = [...order.expense, ...order.income]
    const unchanged =
      ids.join() === [...activeIds(all, 'expense'), ...activeIds(all, 'income')].join()
    if (unchanged) {
      setOrder(null)
      return
    }
    reorder.mutate({ ids })
  }

  function groupOf(kind: CategoryKind): Category[] {
    const ids = order ? order[kind] : activeIds(all, kind)
    return ids.flatMap((id) => byId.get(id) ?? [])
  }

  return (
    <>
      <div className="flex flex-col gap-2">
        <BackLink to="/settings" label={settings.title} />
        <PageTitle>{t.title}</PageTitle>
        <p className="text-muted-foreground">{t.intro}</p>
      </div>
      <Notice>{notice}</Notice>
      {reorder.isError && <FormAlert>{errorText(reorder.error)}</FormAlert>}

      {categories.isPending && <CategoriesSkeleton />}
      {categories.isError && (
        <div className="flex flex-col gap-4">
          <FormAlert>
            <p>{t.loadError}</p>
            <p>{errorText(categories.error)}</p>
          </FormAlert>
          <Button
            variant="outline"
            onClick={() => {
              void categories.refetch()
            }}
          >
            {common.tryAgain}
          </Button>
        </div>
      )}
      {categories.isSuccess && (
        <div className="flex flex-col gap-8">
          {(['expense', 'income'] as const).map((kind) => (
            <CategoryGroup
              key={kind}
              title={kind === 'expense' ? t.expenses : t.income}
              empty={t.noneActive(kind)}
              categories={groupOf(kind)}
              editing={editing}
              onOpen={(category) => {
                setNotice(null)
                setSheetKey((k) => k + 1)
                setEditTarget(category)
                setEditOpen(true)
              }}
              onMove={(index, step) => {
                setOrder((o) => (o ? { ...o, [kind]: moveItem(o[kind], index, step) } : o))
              }}
            />
          ))}
          {!editing && all.some((c) => c.archived) && (
            <CategoryGroup
              title={t.archived}
              empty=""
              categories={all.filter((c) => c.archived)}
              editing={false}
              onOpen={(category) => {
                setNotice(null)
                setSheetKey((k) => k + 1)
                setEditTarget(category)
                setEditOpen(true)
              }}
              onMove={() => undefined}
            />
          )}
        </div>
      )}

      {/* The main actions stay within thumb reach, just above the app bar on a phone. */}
      {categories.isSuccess && (
        <div className="sticky bottom-[calc(4rem+env(safe-area-inset-bottom))] mt-auto grid grid-cols-2 gap-3 bg-background pt-3 pb-4 md:static md:py-0">
          {editing ? (
            <>
              <Button
                variant="outline"
                size="lg"
                onClick={() => {
                  reorder.reset()
                  setOrder(null)
                }}
              >
                {common.cancel}
              </Button>
              <Button
                size="lg"
                disabled={reorder.isPending}
                focusableWhenDisabled
                onClick={saveOrder}
              >
                {reorder.isPending ? t.savingOrder : t.saveOrder}
              </Button>
            </>
          ) : (
            <>
              <Button variant="outline" size="lg" onClick={startEditing}>
                {t.editOrder}
              </Button>
              <Button
                size="lg"
                onClick={() => {
                  setNotice(null)
                  setSheetKey((k) => k + 1)
                  setAddOpen(true)
                }}
              >
                {t.add}
              </Button>
            </>
          )}
        </div>
      )}

      <Sheet key={`add-${String(sheetKey)}`} open={addOpen} onOpenChange={setAddOpen}>
        <SheetContent>
          <AddForm count={all.length} onDone={done} />
        </SheetContent>
      </Sheet>
      <Sheet key={`edit-${String(sheetKey)}`} open={editOpen} onOpenChange={setEditOpen}>
        <SheetContent>
          {editTarget && <EditForm category={editTarget} onDone={done} />}
        </SheetContent>
      </Sheet>
    </>
  )
}

function CategoryGroup({
  title,
  empty,
  categories,
  editing,
  onOpen,
  onMove,
}: {
  title: string
  empty: string
  categories: readonly Category[]
  editing: boolean
  onOpen: (category: Category) => void
  onMove: (index: number, step: -1 | 1) => void
}) {
  return (
    <section className="flex flex-col gap-2">
      <h2 className="text-xl font-semibold">{title}</h2>
      {categories.length === 0 ? (
        <p className="border-y border-border py-4 text-muted-foreground">{empty}</p>
      ) : (
        <ul
          aria-label={title}
          className="flex flex-col divide-y divide-border border-y border-border"
        >
          {categories.map((category, index) => (
            <li key={category.id}>
              {editing ? (
                <div className="flex min-h-14 items-center gap-3 py-2">
                  <CategoryIcon icon={category.icon} />
                  <span className="min-w-0 flex-1 [overflow-wrap:anywhere]">{category.name}</span>
                  <Button
                    variant="ghost"
                    size="icon"
                    aria-label={t.moveUp(category.name)}
                    disabled={index === 0}
                    focusableWhenDisabled
                    onClick={() => {
                      onMove(index, -1)
                    }}
                  >
                    <ArrowUpIcon />
                  </Button>
                  <Button
                    variant="ghost"
                    size="icon"
                    aria-label={t.moveDown(category.name)}
                    disabled={index === categories.length - 1}
                    focusableWhenDisabled
                    onClick={() => {
                      onMove(index, 1)
                    }}
                  >
                    <ArrowDownIcon />
                  </Button>
                </div>
              ) : (
                <button
                  type="button"
                  className="flex min-h-14 w-full items-center gap-3 py-2 text-left select-none active:bg-muted"
                  onClick={() => {
                    onOpen(category)
                  }}
                >
                  <CategoryIcon icon={category.icon} />
                  <span className="min-w-0 flex-1 [overflow-wrap:anywhere]">{category.name}</span>
                  <ChevronRightIcon aria-hidden="true" className="size-5 text-muted-foreground" />
                </button>
              )}
            </li>
          ))}
        </ul>
      )}
    </section>
  )
}

function CategoriesSkeleton() {
  return (
    <div aria-hidden="true" data-testid="categories-skeleton" className="flex flex-col gap-2">
      <Skeleton className="h-6 w-28" />
      {[0, 1, 2, 3].map((i) => (
        <div key={i} className="flex items-center gap-3 py-2">
          <Skeleton className="size-10 rounded-full" />
          <Skeleton className="h-5 w-1/2" />
        </div>
      ))}
    </div>
  )
}

/** Where an error from a category write belongs: the name, the icon, or the whole form. */
function placeError(err: unknown): { name?: string; icon?: string; form?: string } {
  const text = errorText(err)
  if (isApiError(err, 'conflict')) {
    // A taken name belongs on the name; the 200 limit is about the whole list.
    return err.message.includes('limit') ? { form: text } : { name: text }
  }
  if (isApiError(err, 'invalid_input')) {
    if (err.message.startsWith('the name')) {
      return { name: text }
    }
    if (err.message.startsWith('the icon')) {
      return { icon: text }
    }
  }
  return { form: text }
}

function AddForm({ count, onDone }: { count: number; onDone: (notice: string | null) => void }) {
  const [name, setName] = useState('')
  const [kind, setKind] = useState<CategoryKind>('expense')
  const [icon, setIcon] = useState('')
  const [errors, setErrors] = useState<{ name?: string; icon?: string; form?: string }>({})

  const create = useMutation({
    mutationFn: createCategory,
    onSuccess: (category) => {
      onDone(t.created(category.name))
    },
    onError: (err) => {
      setErrors(placeError(err))
    },
  })

  function submit(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault()
    if (create.isPending) {
      return
    }
    if (count >= maxCategories) {
      setErrors({ form: t.limitReached })
      return
    }
    const n = checkCategoryName(name)
    const i = checkCategoryIcon(icon)
    const next = { name: n.ok ? undefined : n.error, icon: i.ok ? undefined : i.error }
    setErrors(next)
    if (!n.ok || !i.ok) {
      return
    }
    create.mutate(i.value === '' ? { name: n.value, kind } : { name: n.value, kind, icon: i.value })
  }

  return (
    <form noValidate onSubmit={submit} className="flex flex-col gap-6">
      <SheetHeader>
        <SheetTitle>{t.addTitle}</SheetTitle>
      </SheetHeader>
      <TextField
        label={t.name}
        hint={t.nameHint}
        name="category-name"
        autoComplete="off"
        autoCapitalize="sentences"
        enterKeyHint="next"
        value={name}
        error={errors.name}
        onChange={(e) => {
          setName(e.target.value)
        }}
      />
      <ChoiceGroup<CategoryKind>
        legend={t.kind}
        showLegend
        choices={[
          { value: 'expense', label: t.expense },
          { value: 'income', label: t.incomeKind },
        ]}
        value={kind}
        onChange={setKind}
      />
      <IconField value={icon} error={errors.icon} onChange={setIcon} />
      {errors.form && <FormAlert>{errors.form}</FormAlert>}
      <SheetFooter className="mt-0">
        <Button type="submit" size="lg" disabled={create.isPending} focusableWhenDisabled>
          {create.isPending ? t.creating : t.create}
        </Button>
      </SheetFooter>
    </form>
  )
}

function IconField({
  value,
  error,
  onChange,
}: {
  value: string
  error: string | undefined
  onChange: (value: string) => void
}) {
  return (
    <TextField
      label={t.icon}
      hint={t.iconHint}
      name="category-icon"
      autoComplete="off"
      autoCorrect="off"
      spellCheck={false}
      enterKeyHint="done"
      inputClassName="text-xl"
      value={value}
      error={error}
      onChange={(e) => {
        onChange(e.target.value)
      }}
    />
  )
}

function EditForm({
  category,
  onDone,
}: {
  category: Category
  onDone: (notice: string | null) => void
}) {
  const [name, setName] = useState(category.name)
  const [icon, setIcon] = useState(category.icon)
  const [errors, setErrors] = useState<{ name?: string; icon?: string; form?: string }>({})

  const save = useMutation({
    mutationFn: (req: UpdateCategoryRequest) => updateCategory(category.id, req),
    onSuccess: (updated, req) => {
      if (req.archived === true) {
        onDone(t.archivedOk(updated.name))
      } else if (req.archived === false) {
        onDone(t.unarchivedOk(updated.name))
      } else {
        onDone(t.saved(updated.name))
      }
    },
    onError: (err, req) => {
      // Unarchiving can clash with an active category's name; that is about the whole action.
      setErrors(req.archived === false ? { form: errorText(err) } : placeError(err))
    },
  })

  function submit(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault()
    if (save.isPending) {
      return
    }
    const n = checkCategoryName(name)
    const i = checkCategoryIcon(icon)
    setErrors({ name: n.ok ? undefined : n.error, icon: i.ok ? undefined : i.error })
    if (!n.ok || !i.ok) {
      return
    }
    const req: UpdateCategoryRequest = {}
    if (n.value !== category.name) {
      req.name = n.value
    }
    if (i.value !== category.icon) {
      req.icon = i.value
    }
    if (Object.keys(req).length === 0) {
      onDone(null)
      return
    }
    save.mutate(req)
  }

  return (
    <form noValidate onSubmit={submit} className="flex flex-col gap-6">
      <SheetHeader>
        <SheetTitle>{t.editTitle}</SheetTitle>
        <SheetDescription>{t.kindFixed(category.kind)}</SheetDescription>
      </SheetHeader>
      <TextField
        label={t.name}
        hint={t.nameHint}
        name="category-name"
        autoComplete="off"
        autoCapitalize="sentences"
        enterKeyHint="next"
        value={name}
        error={errors.name}
        onChange={(e) => {
          setName(e.target.value)
        }}
      />
      <IconField value={icon} error={errors.icon} onChange={setIcon} />
      {errors.form && <FormAlert>{errors.form}</FormAlert>}
      <SheetFooter className="mt-0">
        <Button
          type="button"
          variant="outline"
          size="lg"
          disabled={save.isPending}
          focusableWhenDisabled
          onClick={() => {
            if (!save.isPending) {
              setErrors({})
              save.mutate({ archived: !category.archived })
            }
          }}
        >
          {category.archived
            ? save.isPending && save.variables.archived === false
              ? t.unarchiving
              : t.unarchive
            : save.isPending && save.variables.archived === true
              ? t.archiving
              : t.archive}
        </Button>
        <Button type="submit" size="lg" disabled={save.isPending} focusableWhenDisabled>
          {save.isPending && save.variables.archived === undefined ? t.saving : t.save}
        </Button>
      </SheetFooter>
    </form>
  )
}
