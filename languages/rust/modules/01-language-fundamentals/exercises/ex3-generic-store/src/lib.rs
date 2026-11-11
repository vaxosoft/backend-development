//! Exercise 3: traits, generics, closures and returning iterators.

use std::collections::BTreeMap;

/// Anything stored in a `Store` has a numeric id.
pub trait Entity {
    fn id(&self) -> u32;
}

/// An in-memory collection keyed by `Entity::id`, iterated in id order.
#[derive(Debug)]
pub struct Store<T> {
    items: BTreeMap<u32, T>,
}

impl<T> Default for Store<T> {
    fn default() -> Self {
        Store {
            items: BTreeMap::new(),
        }
    }
}

impl<T: Entity> Store<T> {
    pub fn new() -> Self {
        Self::default()
    }

    pub fn len(&self) -> usize {
        self.items.len()
    }

    pub fn is_empty(&self) -> bool {
        self.items.is_empty()
    }

    pub fn get(&self, id: u32) -> Option<&T> {
        self.items.get(&id)
    }

    /// Stores `item` under its id and returns the item it replaced, if any.
    pub fn insert(&mut self, _item: T) -> Option<T> {
        todo!("BTreeMap::insert already returns the previous value")
    }

    pub fn remove(&mut self, _id: u32) -> Option<T> {
        todo!("remove and return the item")
    }

    /// Applies `change` to the item in place. Returns `false` if `id` is unknown.
    pub fn update(&mut self, _id: u32, _change: impl FnOnce(&mut T)) -> bool {
        todo!("get_mut + call the closure")
    }

    /// Lazily yields the items matching `pred`, in id order. Nothing is cloned.
    pub fn filter<'a>(
        &'a self,
        pred: impl Fn(&T) -> bool + 'a,
    ) -> impl Iterator<Item = &'a T> + 'a {
        // Replace with a lazy `filter` over the stored values.
        let _ = pred;
        std::iter::empty()
    }

    /// One more than the largest id in use (1 for an empty store).
    pub fn next_id(&self) -> u32 {
        todo!("last key + 1")
    }
}

impl<T: Entity> FromIterator<T> for Store<T> {
    /// Later items replace earlier ones with the same id.
    fn from_iter<I: IntoIterator<Item = T>>(_iter: I) -> Self {
        todo!("start from Store::new() and insert every item")
    }
}
