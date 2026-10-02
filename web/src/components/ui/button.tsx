// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Button — shadcn-style Button component (new-york variant). Lightweight
// implementation tuned to SkillForge's design tokens. Uses CSS variables
// defined in src/styles/theme.css.
//
// Variants:
//   - default: bg-primary, text-primary-fg (the main CTA in chat input)
//   - secondary: bg-surface-2, border
//   - ghost: transparent, hover surface-2
//   - destructive: bg-danger (reserved, unused in P4)
// Sizes: default / sm / icon
import { cva, type VariantProps } from 'class-variance-authority';
import { forwardRef, type ButtonHTMLAttributes } from 'react';
import { cn } from '../../lib/cn';

const buttonVariants = cva(
  'inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-md text-xs font-semibold transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary focus-visible:ring-offset-2 focus-visible:ring-offset-bg disabled:pointer-events-none disabled:opacity-50',
  {
    variants: {
      variant: {
        default: 'bg-primary text-primary-fg hover:bg-primary-hover',
        secondary: 'bg-surface-2 text-text border border-border hover:bg-surface-3',
        ghost: 'text-text hover:bg-surface-2',
        destructive: 'bg-danger text-primary-fg hover:opacity-90',
      },
      size: {
        default: 'h-9 px-4',
        sm: 'h-7 px-3 text-xs',
        icon: 'h-8 w-8',
      },
    },
    defaultVariants: {
      variant: 'default',
      size: 'default',
    },
  },
);

export interface ButtonProps
  extends ButtonHTMLAttributes<HTMLButtonElement>,
    VariantProps<typeof buttonVariants> {}

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(
  ({ className, variant, size, ...props }, ref) => {
    return (
      <button
        ref={ref}
        className={cn(buttonVariants({ variant, size, className }))}
        {...props}
      />
    );
  },
);
Button.displayName = 'Button';
