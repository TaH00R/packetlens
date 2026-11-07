import type { ReactNode } from "react";
export default function PageFrame({
  title,
  description,
  children,
}: {
  title: string;
  description: string;
  children: ReactNode;
}) {
  return (
    <div className="mx-auto w-full max-w-[1600px] space-y-7 p-4 sm:p-7 xl:p-9">
      <header className="space-y-2">
        <h1 className="text-2xl font-semibold tracking-tight text-slate-100 sm:text-3xl">
          {title}
        </h1>
        <p className="max-w-3xl text-sm leading-6 text-slate-400">
          {description}
        </p>
      </header>
      {children}
    </div>
  );
}
